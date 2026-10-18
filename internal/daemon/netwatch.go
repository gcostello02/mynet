package daemon

import (
	"context"
	"sync"
	"time"

	"github.com/grantcostello/mynet/internal/client"
)

const defaultNetPollInterval = 15 * time.Second
const netChangeDebounce = 3 * time.Second

// NetFingerprint is a coarse snapshot of the host's default-route egress.
type NetFingerprint string

// NetSnapshotFunc returns a fingerprint or empty if unknown.
type NetSnapshotFunc func() NetFingerprint

// NetWatcher polls for meaningful network changes and notifies the node.
type NetWatcher struct {
	snapshot NetSnapshotFunc
	onChange func()
	interval time.Duration

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
}

func NewNetWatcher(snapshot NetSnapshotFunc, onChange func()) *NetWatcher {
	if snapshot == nil {
		snapshot = func() NetFingerprint { return "" }
	}
	return &NetWatcher{
		snapshot: snapshot,
		onChange: onChange,
		interval: defaultNetPollInterval,
	}
}

func (w *NetWatcher) Start(ctx context.Context) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	cctx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.running = true
	w.mu.Unlock()

	go w.loop(cctx)
}

func (w *NetWatcher) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	if w.cancel != nil {
		w.cancel()
	}
	w.running = false
	w.mu.Unlock()
}

func (w *NetWatcher) loop(ctx context.Context) {
	var last NetFingerprint
	var debounce <-chan time.Time
	for {
		fp := w.snapshot()
		if fp != "" && last != "" && fp != last {
			if debounce == nil {
				debounce = time.After(netChangeDebounce)
			}
		}
		if fp != "" {
			last = fp
		}
		if debounce != nil {
			select {
			case <-ctx.Done():
				return
			case <-debounce:
				debounce = nil
				if w.onChange != nil {
					w.onChange()
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(w.interval):
		}
	}
}

// InvalidatePeerTransports closes cached ICE links so peer workers renegotiate.
func InvalidatePeerTransports(rt *client.Runtime) {
	if rt == nil {
		return
	}
	rt.InvalidateAllPeerTransports()
}
