package client

import (
	"testing"
	"time"
)

func TestDropPeerForRevocationCancelsActiveICEAttempt(t *testing.T) {
	rt := NewRuntime(t.TempDir(), nil)
	rt.SetVPNMode(true)
	ctx, end := rt.beginPeerICEAttempt("client-c")
	rt.DropPeerForRevocation("client-c")
	if ctx.Err() == nil {
		t.Fatal("expected ICE attempt cancelled on revocation teardown")
	}
	end()
	// Ensure a new attempt for another peer is unaffected.
	ctxB, endB := rt.beginPeerICEAttempt("client-b")
	time.Sleep(10 * time.Millisecond)
	if ctxB.Err() != nil {
		t.Fatal("revocation for client-c should not cancel client-b ICE")
	}
	endB()
}
