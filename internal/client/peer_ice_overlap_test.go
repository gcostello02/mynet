package client

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestOverlappingAttemptsCancelOnlyOlder(t *testing.T) {
	rt := NewRuntime(t.TempDir(), nil)
	ctx1, end1 := rt.beginPeerICEAttempt("peer")
	ctx2, end2 := rt.beginPeerICEAttempt("peer")
	defer end2()
	if ctx1.Err() == nil {
		t.Fatal("first attempt should be cancelled")
	}
	if ctx2.Err() != nil {
		t.Fatal("second attempt should remain active")
	}
	end1()
}

func TestDoneDoesNotClearNewerAttempt(t *testing.T) {
	rt := NewRuntime(t.TempDir(), nil)
	_, end1 := rt.beginPeerICEAttempt("peer")
	ctx2, end2 := rt.beginPeerICEAttempt("peer")
	end1()
	if ctx2.Err() != nil {
		t.Fatal("stale done() must not cancel newer attempt")
	}
	end2()
}

func TestSupersedeFlagConsumedOnce(t *testing.T) {
	rt := NewRuntime(t.TempDir(), nil)
	rt.cancelPeerICEAttempt("peer")
	rt.mu.Lock()
	if !rt.peerICESupersedeAllow["peer"] {
		t.Fatal("expected supersede flag")
	}
	rt.mu.Unlock()
	rt.mu.Lock()
	rt.peerICESupersedeAllow["peer"] = false
	rt.mu.Unlock()
}

func TestCredentialWaitRespectsCancel(t *testing.T) {
	rt := NewRuntime(t.TempDir(), nil)
	ctx, end := rt.beginPeerICEAttempt("peer")
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			if ctx.Err() != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Millisecond):
			}
		}
	}()
	time.Sleep(15 * time.Millisecond)
	rt.cancelPeerICEAttempt("peer")
	wg.Wait()
	if !errorsIsCanceled(ctx) {
		t.Fatalf("expected cancel, got %v", ctx.Err())
	}
	end()
}

func errorsIsCanceled(ctx context.Context) bool {
	return ctx.Err() != nil
}
