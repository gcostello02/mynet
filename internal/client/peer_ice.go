package client

import (
	"context"
	"sync/atomic"
)

// cancelPeerICEAttempt aborts an in-flight VPN ICE negotiation for peerName (e.g. before a new CreateSession).
func (r *Runtime) cancelPeerICEAttempt(peerName string) {
	if peerName == "" {
		return
	}
	r.mu.Lock()
	if r.peerICESupersedeAllow == nil {
		r.peerICESupersedeAllow = make(map[string]bool)
	}
	r.peerICESupersedeAllow[peerName] = true
	r.mu.Unlock()
	r.peerICEMu.Lock()
	if attempt, ok := r.peerICECancel[peerName]; ok {
		attempt.cancel()
		delete(r.peerICECancel, peerName)
	}
	r.peerICEMu.Unlock()
}

type peerICEAttempt struct {
	gen    uint64
	cancel context.CancelFunc
}

// beginPeerICEAttempt starts a new ICE attempt for peerName, cancelling any prior attempt.
// The returned done function removes this attempt from the registry when the goroutine exits.
func (r *Runtime) beginPeerICEAttempt(peerName string) (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	gen := atomic.AddUint64(&r.peerICEGen, 1)
	r.peerICEMu.Lock()
	if r.peerICECancel == nil {
		r.peerICECancel = make(map[string]peerICEAttempt)
	}
	if old, ok := r.peerICECancel[peerName]; ok {
		old.cancel()
	}
	r.peerICECancel[peerName] = peerICEAttempt{gen: gen, cancel: cancel}
	r.peerICEMu.Unlock()

	done := func() {
		r.peerICEMu.Lock()
		if cur, ok := r.peerICECancel[peerName]; ok && cur.gen == gen {
			delete(r.peerICECancel, peerName)
		}
		r.peerICEMu.Unlock()
	}
	return ctx, done
}
