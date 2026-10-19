package client

import (
	"net"
	"testing"

	"github.com/gcostello02/mynet/internal/control"
)

func TestDuplicateSessionReadyAlreadyHandling(t *testing.T) {
	rt := NewRuntime(t.TempDir(), nil)
	rt.mu.Lock()
	rt.incomingICE["s1"] = true
	rt.mu.Unlock()
	ok := rt.beginIncomingICESession(control.SessionReadyPayload{SessionID: "s1", PeerName: "peer"})
	if ok {
		t.Fatal("expected duplicate to be skipped")
	}
}

func TestVPNAlreadyTransportSkipsSessionReady(t *testing.T) {
	rt := NewRuntime(t.TempDir(), nil)
	rt.SetVPNMode(true)
	rt.peerLinks = map[string]*PeerLink{
		"macbook": {PeerName: "macbook", SessionID: "s-live", Conn: &net.TCPConn{}},
	}
	ok := rt.beginIncomingICESession(control.SessionReadyPayload{SessionID: "s-live", PeerName: "macbook", Role: "controlled"})
	if ok {
		t.Fatal("expected idempotent skip for live transport")
	}
}

func TestStaleSessionInflightRejected(t *testing.T) {
	rt := NewRuntime(t.TempDir(), nil)
	rt.mu.Lock()
	rt.peerInFlightICE["macbook"] = "s-new"
	rt.incomingICE["s-new"] = true
	rt.mu.Unlock()
	ok := rt.beginIncomingICESession(control.SessionReadyPayload{SessionID: "s-old", PeerName: "macbook", Role: "controlling"})
	if ok {
		t.Fatal("expected stale in-flight session to be rejected for controlling without supersede flag")
	}
}

func TestControlledSupersedesInflightSession(t *testing.T) {
	rt := NewRuntime(t.TempDir(), nil)
	rt.mu.Lock()
	rt.peerInFlightICE["mini"] = "s-old"
	rt.incomingICE["s-old"] = true
	rt.mu.Unlock()
	ok := rt.beginIncomingICESession(control.SessionReadyPayload{SessionID: "s-new", PeerName: "mini", Role: "controlled"})
	if !ok {
		t.Fatal("controlled peer should accept newer coordinator session")
	}
	rt.mu.Lock()
	got := rt.peerInFlightICE["mini"]
	rt.mu.Unlock()
	if got != "s-new" {
		t.Fatalf("inflight %q", got)
	}
}

func TestInitiatorSupersedeFlagAllowsReplacement(t *testing.T) {
	rt := NewRuntime(t.TempDir(), nil)
	rt.mu.Lock()
	rt.peerInFlightICE["macbook"] = "s-old"
	rt.incomingICE["s-old"] = true
	rt.mu.Unlock()
	rt.cancelPeerICEAttempt("macbook")
	ok := rt.beginIncomingICESession(control.SessionReadyPayload{SessionID: "s-new", PeerName: "macbook", Role: "controlling"})
	if !ok {
		t.Fatal("initiator supersede should allow new controlling session")
	}
}
