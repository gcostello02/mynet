package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestHubUnregisterDoesNotRemoveReplacement(t *testing.T) {
	h := NewHub(slog.Default())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader.Upgrade(w, r, nil)
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	c1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	id1 := h.Register("dev1", c1)
	if !h.IsOnline("dev1") {
		t.Fatal("dev1 should be online")
	}

	c2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	id2 := h.Register("dev1", c2)
	if id1 == id2 {
		t.Fatal("connection ids must differ")
	}
	if !h.IsOnline("dev1") {
		t.Fatal("dev1 should stay online after replacement")
	}

	h.Unregister("dev1", id1)
	if !h.IsOnline("dev1") {
		t.Fatal("stale unregister must not clear active connection")
	}

	h.Unregister("dev1", id2)
	if h.IsOnline("dev1") {
		t.Fatal("active unregister should clear presence")
	}
	_ = c1.Close()
	_ = c2.Close()
}

func TestHubStaleCloseAfterReplacement(t *testing.T) {
	h := NewHub(slog.Default())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader.Upgrade(w, r, nil)
	}))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	first, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	firstID := h.Register("dev", first)

	second, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	secondID := h.Register("dev", second)
	_ = first.Close()
	h.Unregister("dev", firstID)
	time.Sleep(20 * time.Millisecond)
	if !h.IsOnline("dev") {
		t.Fatal("replacement connection must remain online when stale socket closes")
	}
	h.Unregister("dev", secondID)
	_ = second.Close()
}
