package client

import (
	"testing"
	"time"
)

func TestWaitPeerGatherGraceExitsEarlyOnPeerDone(t *testing.T) {
	sig := &signalState{}
	sig.peerGatherDone = true
	start := time.Now()
	if err := waitPeerGatherGrace(sig, 500*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 50*time.Millisecond {
		t.Fatalf("expected immediate return, took %v", time.Since(start))
	}
}

func TestWaitPeerGatherGraceWaitsUntilGrace(t *testing.T) {
	sig := &signalState{}
	start := time.Now()
	_ = waitPeerGatherGrace(sig, 80*time.Millisecond)
	elapsed := time.Since(start)
	if elapsed < 70*time.Millisecond {
		t.Fatalf("expected ~80ms grace, got %v", elapsed)
	}
}
