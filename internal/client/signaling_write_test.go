package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/grantcostello/mynet/internal/control"
)

func dialTestWS(t *testing.T, onMsg func()) *websocket.Conn {
	up := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		for {
			_, data, err := c.ReadMessage()
			if err != nil {
				return
			}
			if onMsg != nil {
				onMsg()
			}
			var msg control.WSMessage
			if json.Unmarshal(data, &msg) != nil || msg.SessionID == "" {
				t.Errorf("invalid ws message")
			}
		}
	}))
	t.Cleanup(srv.Close)
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestSendSignalConcurrentThreeSessions(t *testing.T) {
	conn := dialTestWS(t, nil)
	rt := NewRuntime(t.TempDir(), slog.Default())
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	rt.replaceSignalingWebSocket(conn, cancel)

	const sessions = 3
	const n = 80
	var ok atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < sessions; i++ {
		sid := fmt.Sprintf("session-%d", i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < n; j++ {
				err := rt.sendSignal(sid, control.WSMessage{
					Type:    control.SignalICECandidate,
					Payload: control.ICECandidatePayload{Candidate: "candidate:1 1 udp 1 1.1.1.1 9 typ host"},
				})
				if err != nil {
					t.Errorf("sendSignal: %v", err)
					return
				}
				ok.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := ok.Load(); got != int64(sessions*n) {
		t.Fatalf("expected %d successful sends, got %d", sessions*n, got)
	}
}

func TestSendSignalDuringReconnect(t *testing.T) {
	conn1 := dialTestWS(t, nil)
	conn2 := dialTestWS(t, nil)
	rt := NewRuntime(t.TempDir(), slog.Default())
	_, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	rt.replaceSignalingWebSocket(conn1, cancel1)

	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = rt.sendSignal("s1", control.WSMessage{Type: control.SignalGatherDone})
			}
		}
	}()

	time.Sleep(20 * time.Millisecond)
	_, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	rt.replaceSignalingWebSocket(conn2, cancel2)
	close(stop)
	wg.Wait()

	if err := rt.sendSignal("s1", control.WSMessage{Type: control.SignalGatherDone}); err != nil {
		t.Fatalf("write on new conn: %v", err)
	}
}

func TestSendSignalAfterClearReturnsError(t *testing.T) {
	conn := dialTestWS(t, nil)
	rt := NewRuntime(t.TempDir(), slog.Default())
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	rt.replaceSignalingWebSocket(conn, cancel)
	rt.clearSignalingWebSocket(conn)
	if err := rt.sendSignal("s", control.WSMessage{Type: control.SignalGatherDone}); err == nil {
		t.Fatal("expected error after clear")
	}
}
