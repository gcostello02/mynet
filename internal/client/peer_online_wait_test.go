package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/grantcostello/mynet/internal/control"
)

func TestWaitPeerOnlineForICERespectsContext(t *testing.T) {
	rt := NewRuntime(t.TempDir(), nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	rt.waitPeerOnlineForICE(ctx, "peer-b")
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("expected immediate return on canceled ctx, took %v", elapsed)
	}
}

func TestWaitPeerOnlineForICEExitsWhenPeerOnline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]control.PeerInfo{{Name: "peer-a", Online: true}})
	}))
	defer srv.Close()
	rt := NewRuntime(t.TempDir(), nil)
	rt.api = control.NewAPIClient(srv.URL)
	rt.api.Token = "test"
	start := time.Now()
	rt.waitPeerOnlineForICE(context.Background(), "peer-a")
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("expected fast return when peer already online, took %v", elapsed)
	}
}

func TestWaitPeerOnlineForICEBounded(t *testing.T) {
	prev := peerOnlineICEMaxWait
	peerOnlineICEMaxWait = 200 * time.Millisecond
	defer func() { peerOnlineICEMaxWait = prev }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]control.PeerInfo{{Name: "peer-z", Online: false}})
	}))
	defer srv.Close()
	rt := NewRuntime(t.TempDir(), nil)
	rt.api = control.NewAPIClient(srv.URL)
	rt.api.Token = "test"
	start := time.Now()
	rt.waitPeerOnlineForICE(context.Background(), "peer-z")
	elapsed := time.Since(start)
	if elapsed < 180*time.Millisecond || elapsed > 800*time.Millisecond {
		t.Fatalf("expected ~200ms bounded wait, got %v", elapsed)
	}
}
