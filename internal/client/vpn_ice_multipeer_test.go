package client

import (
	"sync/atomic"
	"testing"

	"github.com/grantcostello/mynet/internal/control"
)

// VPN daemons negotiate ICE to multiple peers concurrently; cancelling one peer's dial
// must not abort another peer's in-flight session.
func TestVPNModeDoesNotCancelOtherPeerICEDial(t *testing.T) {
	rt := NewRuntime(t.TempDir(), nil)
	rt.SetVPNMode(true)

	var cancels atomic.Int32
	rt.mu.Lock()
	rt.iceCancel = func() { cancels.Add(1) }
	rt.iceCancelFor = "session-a"
	rt.iceDialPeer = "peer-a"
	rt.peerName = "peer-a"
	rt.activeSession = "session-a"
	rt.mu.Unlock()

	readyB := control.SessionReadyPayload{
		SessionID: "session-b",
		PeerName:  "peer-b",
		Role:      "controlled",
	}
	if !rt.beginIncomingICESession(readyB) {
		t.Fatal("expected session-b to be accepted")
	}
	if cancels.Load() != 0 {
		t.Fatalf("expected no ICE cancel for different peer, got %d", cancels.Load())
	}

	rt.mu.Lock()
	if rt.peerName != "peer-b" {
		t.Fatalf("expected active peer peer-b, got %q", rt.peerName)
	}
	if rt.activeSession != "session-b" {
		t.Fatalf("expected session-b active, got %q", rt.activeSession)
	}
	rt.mu.Unlock()
}

func TestThreePeerICEAttemptsIndependent(t *testing.T) {
	rt := NewRuntime(t.TempDir(), nil)
	rt.SetVPNMode(true)
	ctxA, endA := rt.beginPeerICEAttempt("client-a")
	ctxB, endB := rt.beginPeerICEAttempt("client-b")
	ctxC, endC := rt.beginPeerICEAttempt("client-c")
	if ctxA.Err() != nil || ctxB.Err() != nil || ctxC.Err() != nil {
		t.Fatal("all peer ICE attempts should be active")
	}
	rt.cancelPeerICEAttempt("client-b")
	if ctxB.Err() == nil {
		t.Fatal("client-b attempt should cancel")
	}
	if ctxA.Err() != nil || ctxC.Err() != nil {
		t.Fatal("cancel must not affect other peers")
	}
	endA()
	endC()
	endB()
}

func TestVPNModeCancelsSamePeerNewSession(t *testing.T) {
	rt := NewRuntime(t.TempDir(), nil)
	rt.SetVPNMode(true)

	var cancels atomic.Int32
	rt.mu.Lock()
	rt.iceCancel = func() { cancels.Add(1) }
	rt.iceCancelFor = "session-a"
	rt.iceDialPeer = "peer-a"
	rt.peerName = "peer-a"
	rt.activeSession = "session-a"
	rt.mu.Unlock()

	ready := control.SessionReadyPayload{
		SessionID: "session-a2",
		PeerName:  "peer-a",
		Role:      "controlled",
	}
	if !rt.beginIncomingICESession(ready) {
		t.Fatal("expected new session for same peer")
	}
	if cancels.Load() != 1 {
		t.Fatalf("expected cancel for superseding same peer, got %d", cancels.Load())
	}
}
