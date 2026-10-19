package client

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestBeginPeerICEAttemptSupersedesPrior(t *testing.T) {
	rt := NewRuntime(t.TempDir(), nil)
	ctx1, end1 := rt.beginPeerICEAttempt("macbook")
	ctx2, end2 := rt.beginPeerICEAttempt("macbook")
	defer end2()
	if ctx1.Err() == nil {
		t.Fatal("expected prior attempt cancelled")
	}
	if ctx2.Err() != nil {
		t.Fatal("new attempt should be active")
	}
	end1()
}

func TestCancelPeerICEAttemptAbortsWait(t *testing.T) {
	rt := NewRuntime(t.TempDir(), nil)
	ctx, end := rt.beginPeerICEAttempt("peer")
	defer end()
	rt.cancelPeerICEAttempt("peer")
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("expected cancel, got %v", ctx.Err())
	}
}

func TestSupersededICEExitsCredentialWaitQuickly(t *testing.T) {
	rt := NewRuntime(t.TempDir(), nil)
	ctx, end := rt.beginPeerICEAttempt("peer")
	start := time.Now()
	go func() {
		time.Sleep(20 * time.Millisecond)
		rt.cancelPeerICEAttempt("peer")
	}()
	deadline := time.Now().Add(iceConnectTimeoutForTest())
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	end()
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("superseded wait took too long: %v", elapsed)
	}
	if ctx.Err() == nil {
		t.Fatal("expected cancellation")
	}
}

func iceConnectTimeoutForTest() time.Duration {
	return 500 * time.Millisecond
}

func TestVPNICEFailureClearsConnectingState(t *testing.T) {
	rt := NewRuntime(t.TempDir(), nil)
	rt.SetVPNMode(true)
	rt.mu.Lock()
	rt.activeSession = "sess-old"
	rt.connState = StateConnecting
	rt.mu.Unlock()

	rt.failPeerVPN("macbook", errors.New("timed out waiting for peer ICE credentials"))
	rt.mu.Lock()
	rt.activeSession = ""
	rt.connState = StateOnline
	rt.mu.Unlock()

	if rt.activeSession != "" {
		t.Fatalf("activeSession %q", rt.activeSession)
	}
	if rt.connState != StateOnline {
		t.Fatalf("connState %q", rt.connState)
	}
}
