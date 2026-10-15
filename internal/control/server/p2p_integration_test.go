package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/grantcostello/mynet/internal/control"
	icepkg "github.com/grantcostello/mynet/internal/ice"
	"github.com/grantcostello/mynet/internal/peer"
)

func TestICEP2PThroughSignaling(t *testing.T) {
	srv := testServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	_, tokA := enrollDevice(t, srv, "alpha")
	_, tokB := enrollDevice(t, srv, "bravo")

	wsA := dialWS(t, ts.URL, tokA)
	defer wsA.Close()
	wsB := dialWS(t, ts.URL, tokB)
	defer wsB.Close()

	sessionID := createSessionHTTP(t, ts.URL, tokA, "bravo")

	var connA, connB net.Conn
	var errA, errB error
	ready := make(chan struct{}, 2)

	go func() {
		connA, errA = runICEPeer(t, wsA, sessionID, true)
		ready <- struct{}{}
	}()
	go func() {
		connB, errB = runICEPeer(t, wsB, sessionID, false)
		ready <- struct{}{}
	}()
	<-ready
	<-ready

	if errA != nil {
		t.Fatal("peer A:", errA)
	}
	if errB != nil {
		t.Fatal("peer B:", errB)
	}
	if connA == nil || connB == nil {
		t.Fatal("missing ice connection")
	}

	stopB := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopB:
				return
			default:
			}
			f, err := peer.ReadFrame(connB, time.Second)
			if err != nil {
				continue
			}
			if resp := peer.RespondPing(f); resp != nil {
				_, _ = connB.Write(resp)
			}
		}
	}()

	rtt, err := peer.PingRTT(connA, 5*time.Second)
	close(stopB)
	if err != nil {
		t.Fatal(err)
	}
	if rtt <= 0 {
		t.Fatal("expected positive RTT")
	}
}

func dialWS(t *testing.T, baseURL, token string) *websocket.Conn {
	wsURL := strings.Replace(baseURL, "http://", "ws://", 1) + "/v1/ws?token=" + token
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	return conn
}

func createSessionHTTP(t *testing.T, baseURL, token, peerName string) string {
	body := bytes.NewBufferString(`{"peer_name":"` + peerName + `"}`)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/sessions", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("session status %d", resp.StatusCode)
	}
	var out control.CreateSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out.SessionID
}

func runICEPeer(t *testing.T, ws *websocket.Conn, sessionID string, controlling bool) (net.Conn, error) {
	if !waitSessionReady(t, ws, sessionID) {
		return nil, nil
	}
	sess, err := icepkg.NewSession(icepkg.SessionConfig{Controlling: controlling})
	if err != nil {
		return nil, err
	}

	remoteCred := make(chan control.ICECredentialsPayload, 1)
	done := make(chan struct{})
	var relayWG sync.WaitGroup
	relayWG.Add(1)
	go func() {
		defer relayWG.Done()
		relaySignals(ws, sessionID, sess, remoteCred, done)
	}()
	defer func() {
		close(done)
		relayWG.Wait()
	}()

	if err := sess.SetCandidateHandler(func(c string) {
		if c == "" {
			_ = writeWS(ws, control.WSMessage{Type: control.SignalGatherDone, SessionID: sessionID})
			return
		}
		_ = writeWS(ws, control.WSMessage{
			Type:      control.SignalICECandidate,
			SessionID: sessionID,
			Payload:   control.ICECandidatePayload{Candidate: c},
		})
	}); err != nil {
		return nil, err
	}

	ufrag, pwd, err := sess.LocalCredentials()
	if err != nil {
		return nil, err
	}
	if err := writeWS(ws, control.WSMessage{
		Type:      control.SignalICECredentials,
		SessionID: sessionID,
		Payload:   control.ICECredentialsPayload{Ufrag: ufrag, Pwd: pwd},
	}); err != nil {
		return nil, err
	}

	var remote control.ICECredentialsPayload
	select {
	case remote = <-remoteCred:
	case <-time.After(5 * time.Second):
		return nil, context.DeadlineExceeded
	}

	if err := sess.Gather(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	conn, err := sess.Connect(ctx, remote.Ufrag, remote.Pwd)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func waitSessionReady(t *testing.T, ws *websocket.Conn, sessionID string) bool {
	type result struct {
		ok bool
	}
	ch := make(chan result, 1)
	go func() {
		for {
			_, data, err := ws.ReadMessage()
			if err != nil {
				ch <- result{false}
				return
			}
			var msg control.WSMessage
			if json.Unmarshal(data, &msg) != nil {
				continue
			}
			if msg.Type == control.SignalSessionReady && msg.SessionID == sessionID {
				ch <- result{true}
				return
			}
		}
	}()
	select {
	case r := <-ch:
		return r.ok
	case <-time.After(5 * time.Second):
		_ = ws.Close()
		t.Fatalf("timeout waiting session_ready")
		return false
	}
}

func relaySignals(ws *websocket.Conn, sessionID string, sess *icepkg.Session, credCh chan control.ICECredentialsPayload, done <-chan struct{}) {
	stop := make(chan struct{})
	go func() {
		select {
		case <-done:
			_ = ws.Close()
		case <-stop:
		}
	}()
	defer close(stop)

	for {
		_, data, err := ws.ReadMessage()
		if err != nil {
			return
		}
		var msg control.WSMessage
		if json.Unmarshal(data, &msg) != nil || msg.SessionID != sessionID {
			continue
		}
		switch msg.Type {
		case control.SignalICECredentials:
			b, _ := json.Marshal(msg.Payload)
			var c control.ICECredentialsPayload
			if json.Unmarshal(b, &c) == nil {
				select {
				case credCh <- c:
				default:
				}
			}
		case control.SignalICECandidate:
			b, _ := json.Marshal(msg.Payload)
			var c control.ICECandidatePayload
			if json.Unmarshal(b, &c) == nil {
				_ = sess.AddRemoteCandidate(c.Candidate)
			}
		}
	}
}

func writeWS(ws *websocket.Conn, msg control.WSMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return ws.WriteMessage(websocket.TextMessage, data)
}
