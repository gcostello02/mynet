package client

import (
	"sync"
	"time"
)

// RecoveryTimeline tracks wall-clock recovery across multiple ICE attempts for one peer.
type RecoveryTimeline struct {
	State                 string // idle | recovering | stable
	CycleStartedAt        time.Time
	FirstFailureAt        time.Time
	LastFailureAt         time.Time
	SuccessAt             time.Time
	TotalToSuccess        time.Duration
	ICEAttemptsTotal      int
	ICEAttemptsFailed     int
	TransportRecoveries   int
	LastFailureReason     string
	SuccessfulAttempt     int
	LastSuccessfulAttempt time.Duration
}

type recoveryTracker struct {
	mu    sync.Mutex
	peers map[string]*RecoveryTimeline
}

func newRecoveryTracker() *recoveryTracker {
	return &recoveryTracker{peers: make(map[string]*RecoveryTimeline)}
}

func (rt *recoveryTracker) beginCycle(peer string) {
	rt.mu.Lock()
	t := rt.peers[peer]
	if t == nil {
		t = &RecoveryTimeline{State: "recovering"}
		rt.peers[peer] = t
	}
	if t.State == "stable" && !t.SuccessAt.IsZero() {
		t = &RecoveryTimeline{State: "recovering", CycleStartedAt: time.Now()}
		rt.peers[peer] = t
	} else if t.CycleStartedAt.IsZero() {
		t.CycleStartedAt = time.Now()
		t.State = "recovering"
	}
	rt.mu.Unlock()
}

func (rt *recoveryTracker) noteAttempt(peer string) {
	rt.mu.Lock()
	t := rt.ensure(peer)
	t.ICEAttemptsTotal++
	rt.mu.Unlock()
}

func (rt *recoveryTracker) noteFailure(peer, reason string) {
	rt.mu.Lock()
	t := rt.ensure(peer)
	t.ICEAttemptsFailed++
	t.LastFailureReason = reason
	now := time.Now()
	if t.FirstFailureAt.IsZero() {
		t.FirstFailureAt = now
	}
	t.LastFailureAt = now
	t.State = "recovering"
	rt.mu.Unlock()
}

func (rt *recoveryTracker) noteSuccess(peer string, attemptNum int, attemptDuration time.Duration) {
	rt.mu.Lock()
	t := rt.ensure(peer)
	now := time.Now()
	t.SuccessAt = now
	t.SuccessfulAttempt = attemptNum
	t.LastSuccessfulAttempt = attemptDuration
	if !t.CycleStartedAt.IsZero() {
		t.TotalToSuccess = now.Sub(t.CycleStartedAt)
	}
	t.State = "stable"
	rt.mu.Unlock()
}

func (rt *recoveryTracker) noteTransportRecovery(peer string) {
	rt.mu.Lock()
	t := rt.ensure(peer)
	t.TransportRecoveries++
	t.State = "recovering"
	if t.CycleStartedAt.IsZero() {
		t.CycleStartedAt = time.Now()
	}
	rt.mu.Unlock()
}

func (rt *recoveryTracker) snapshot(peer string) (RecoveryTimeline, bool) {
	rt.mu.Lock()
	t, ok := rt.peers[peer]
	if !ok || t == nil {
		rt.mu.Unlock()
		return RecoveryTimeline{}, false
	}
	out := *t
	rt.mu.Unlock()
	return out, true
}

func (rt *recoveryTracker) ensure(peer string) *RecoveryTimeline {
	t := rt.peers[peer]
	if t == nil {
		t = &RecoveryTimeline{State: "recovering", CycleStartedAt: time.Now()}
		rt.peers[peer] = t
	}
	return t
}

// BeginPeerRecoveryCycle marks the start of a connect/recovery cycle for a peer.
func (r *Runtime) BeginPeerRecoveryCycle(peer string) {
	if r.recovery == nil {
		return
	}
	r.recovery.beginCycle(peer)
}

// NoteICEAttemptStarted increments total ICE attempts for recovery metrics.
func (r *Runtime) NoteICEAttemptStarted(peer string) {
	if r.recovery == nil {
		return
	}
	r.recovery.noteAttempt(peer)
}

// NoteICEAttemptFailed records a failed ICE attempt reason.
func (r *Runtime) NoteICEAttemptFailed(peer string, reason string) {
	if r.recovery == nil {
		return
	}
	r.recovery.noteFailure(peer, reason)
}

// NoteICEAttemptSucceeded records successful recovery timing.
func (r *Runtime) NoteICEAttemptSucceeded(peer string, attemptNum int, attemptDuration time.Duration) {
	if r.recovery == nil {
		return
	}
	r.recovery.noteSuccess(peer, attemptNum, attemptDuration)
}

// NoteTransportRecovery records ICE transport loss and re-connect scheduling.
func (r *Runtime) NoteTransportRecovery(peer string) {
	if r.recovery == nil {
		return
	}
	r.recovery.noteTransportRecovery(peer)
}

// PeerRecoveryTimeline returns recovery wall-clock metrics for diagnostics.
func (r *Runtime) PeerRecoveryTimeline(peer string) (RecoveryTimeline, bool) {
	if r.recovery == nil {
		return RecoveryTimeline{}, false
	}
	return r.recovery.snapshot(peer)
}
