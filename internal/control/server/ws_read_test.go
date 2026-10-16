package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gcostello02/mynet/internal/control"
	icepkg "github.com/gcostello02/mynet/internal/ice"
)

func TestRelaySignalsExitsWithoutPanicOnAbruptClose(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		time.Sleep(30 * time.Millisecond)
		_ = c.Close()
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}

	sess, err := icepkg.NewSession(icepkg.SessionConfig{})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	sig := &icePeerSig{remoteCred: make(chan control.ICECredentialsPayload, 1)}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		relaySignals(conn, "sess", sess, sig, done)
	}()

	time.Sleep(100 * time.Millisecond)
	close(done)
	wg.Wait()
}

func TestRelaySignalsDoesNotPollWithReadDeadline(t *testing.T) {
	// Gorilla marks the connection failed after a deadline timeout; polling ReadMessage panics.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := upgrader.Upgrade(w, r, nil)
		if c != nil {
			time.Sleep(500 * time.Millisecond)
			_ = c.WriteMessage(websocket.TextMessage, []byte(`{"type":"ice_candidate","session_id":"sess","payload":{}}`))
		}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	sess, err := icepkg.NewSession(icepkg.SessionConfig{})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	sig := &icePeerSig{remoteCred: make(chan control.ICECredentialsPayload, 1)}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		relaySignals(conn, "sess", sess, sig, done)
	}()
	time.Sleep(600 * time.Millisecond)
	close(done)
	wg.Wait()
}
