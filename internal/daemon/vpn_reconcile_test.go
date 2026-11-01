package daemon

import (
	"sync"
	"testing"
	"time"
)

func TestReconcileMutexPreventsOverlap(t *testing.T) {
	n := &Node{reconcileMu: sync.Mutex{}}
	if !n.reconcileMu.TryLock() {
		t.Fatal("expected lock")
	}
	if n.reconcileMu.TryLock() {
		t.Fatal("expected second trylock to fail")
	}
	n.reconcileMu.Unlock()
}

func TestReconcileIntervalDocumented(t *testing.T) {
	if VPNReconcileInterval != 45*time.Second {
		t.Fatalf("expected 45s reconcile interval, got %v", VPNReconcileInterval)
	}
}
