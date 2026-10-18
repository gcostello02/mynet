package daemon

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gcostello02/mynet/internal/client"
)

func TestNetWatcherDebouncesChange(t *testing.T) {
	var seq atomic.Uint32
	snap := func() NetFingerprint {
		if seq.Load() == 0 {
			return NetFingerprint("a")
		}
		return NetFingerprint("b")
	}
	var changes atomic.Uint32
	w := NewNetWatcher(snap, func() { changes.Add(1) })
	w.interval = 20 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.Start(ctx)

	time.Sleep(30 * time.Millisecond)
	seq.Store(1)
	time.Sleep(4 * time.Second)
	w.Stop()
	if changes.Load() < 1 {
		t.Fatalf("expected at least one network change callback, got %d", changes.Load())
	}
}

func TestInvalidatePeerTransports(t *testing.T) {
	rt := client.NewRuntime(t.TempDir(), nil)
	// no links — should not panic
	InvalidatePeerTransports(rt)
}
