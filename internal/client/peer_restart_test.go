package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gcostello02/mynet/internal/control"
)

// Simulates a restarting initiator issuing a new CreateSession while a prior ICE wait is still active.
func TestCreateSessionCancelsInFlightPeerICE(t *testing.T) {
	var online atomic.Bool
	online.Store(true)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/peers":
			_ = json.NewEncoder(w).Encode([]control.PeerInfo{{Name: "macbook", Online: online.Load()}})
		case "/v1/sessions":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(control.CreateSessionResponse{SessionID: "s1", PeerName: "macbook"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	rt := NewRuntime(t.TempDir(), nil)
	rt.api = control.NewAPIClient(srv.URL)
	rt.api.Token = "tok"
	rt.SetVPNMode(true)

	ctx1, end1 := rt.beginPeerICEAttempt("macbook")
	start := time.Now()
	rt.cancelPeerICEAttempt("macbook")
	end1()
	if time.Since(start) > time.Second {
		t.Fatal("cancel should be immediate")
	}
	if ctx1.Err() == nil {
		t.Fatal("expected cancelled context")
	}

	rt.waitPeerOnlineForICE(context.Background(), "macbook")
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("peer-online wait should be instant when online, took %v", elapsed)
	}
}
