package client

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gcostello02/mynet/internal/control"
)

// Peer B restarts and creates a new session while A still caches the pre-restart transport.
func TestNewSessionSupersedesZombieVPNLink(t *testing.T) {
	rt := NewRuntime(t.TempDir(), nil)
	rt.SetVPNMode(true)
	rt.peerLinks = map[string]*PeerLink{
		"minipc": {
			PeerName:  "minipc",
			SessionID: "7190ec68-63ce-4d7f-8122-666492f35018",
			Conn:      &net.TCPConn{},
		},
	}
	ok := rt.beginIncomingICESession(control.SessionReadyPayload{
		SessionID: "118a0e49-6d62-472f-8758-97736bc94ce8",
		PeerName:  "minipc",
		Role:      "controlled",
	})
	if !ok {
		t.Fatal("expected new coordinator session to supersede cached transport")
	}
	if _, still := rt.peerLinks["minipc"]; still {
		t.Fatal("expected old peer link removed before ICE")
	}
}

func TestPeerOfflineInvalidatesCachedLink(t *testing.T) {
	var online atomic.Bool
	online.Store(true)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]control.PeerInfo{{Name: "minipc", Online: online.Load()}})
	}))
	defer srv.Close()

	rt := NewRuntime(t.TempDir(), nil)
	rt.SetVPNMode(true)
	rt.api = control.NewAPIClient(srv.URL)
	rt.api.Token = "tok"
	rt.peerLinks = map[string]*PeerLink{
		"minipc": {PeerName: "minipc", SessionID: "old", Conn: &net.TCPConn{}},
	}
	rt.notePeerOnlineState("minipc", true)
	online.Store(false)
	rt.syncPeerPresence("minipc")
	if _, ok := rt.peerLinks["minipc"]; ok {
		t.Fatal("expected link cleared after coordinator offline")
	}
}

func TestRecoveryTimelineResetsCycleOnTransportRecoveryAfterStable(t *testing.T) {
	rt := newRecoveryTracker()
	rt.beginCycle("peer")
	rt.noteAttempt("peer")
	rt.noteICESuccess("peer", 1, 50)
	if rt.peers["peer"].State != "stable" {
		t.Fatalf("state %q", rt.peers["peer"].State)
	}
	start := rt.peers["peer"].CycleStartedAt
	rt.noteTransportRecovery("peer")
	if rt.peers["peer"].CycleStartedAt.Equal(start) {
		t.Fatal("expected new recovery cycle after transport-down from stable")
	}
}
