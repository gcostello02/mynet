package client

import (
	"sync"
	"time"
)

// ConnStage marks a point in peer connection establishment (monotonic elapsed since attempt start).
type ConnStage string

const (
	StageSignalingReady       ConnStage = "signaling_ready"
	StagePeerSessionCreated   ConnStage = "peer_session_created"
	StageRemoteCredReceived   ConnStage = "remote_credentials_received"
	StageGatherStarted        ConnStage = "gather_started"
	StageGatherCompleted      ConnStage = "gather_completed"
	StageRemoteCandidate      ConnStage = "remote_candidate"
	StageICEConnectStarted    ConnStage = "ice_connect_started"
	StageICEConnected         ConnStage = "ice_connected"
	StageDataTransportReady   ConnStage = "data_transport_ready"
	StageRecoveryStarted      ConnStage = "recovery_started"
	StageRecoveryCompleted    ConnStage = "recovery_completed"
)

// ConnTimingSnapshot is a single connection attempt timeline (durations from attempt start).
type ConnTimingSnapshot struct {
	PeerName      string
	SessionID     string
	Attempt       int
	StartedAt     time.Time
	Stages        map[ConnStage]time.Duration
	LastStage     ConnStage
	LastStageAt   time.Duration
	AttemptFailed bool
	LastFailure   string
	CompletedAt   time.Time
}

// PeerConnTiming is the latest timing and error history for one VPN peer.
type PeerConnTiming struct {
	Current      ConnTimingSnapshot
	LastSuccess  ConnTimingSnapshot
	LastFailure  ConnTimingSnapshot
	RecoveryCount int
}

type connAttempt struct {
	peer      string
	sessionID string
	attempt   int
	start     time.Time
	stages    map[ConnStage]time.Duration
}

// connTiming tracks per-peer connection establishment metrics.
type connTiming struct {
	mu    sync.Mutex
	peers map[string]*PeerConnTiming
}

func newConnTiming() *connTiming {
	return &connTiming{peers: make(map[string]*PeerConnTiming)}
}

func (ct *connTiming) beginAttempt(peer, sessionID string) *connAttempt {
	ct.mu.Lock()
	pt, ok := ct.peers[peer]
	if !ok {
		pt = &PeerConnTiming{}
		ct.peers[peer] = pt
	}
	attemptNum := 1
	if pt.Current.SessionID != "" {
		attemptNum = pt.Current.Attempt + 1
	}
	rec := &connAttempt{
		peer:      peer,
		sessionID: sessionID,
		attempt:   attemptNum,
		start:     time.Now(),
		stages:    make(map[ConnStage]time.Duration),
	}
	pt.Current = ConnTimingSnapshot{
		PeerName:  peer,
		SessionID: sessionID,
		Attempt:   attemptNum,
		StartedAt: rec.start,
		Stages:    rec.stages,
	}
	ct.mu.Unlock()
	return rec
}

func (ct *connTiming) mark(rec *connAttempt, stage ConnStage) {
	if rec == nil {
		return
	}
	elapsed := time.Since(rec.start)
	ct.mu.Lock()
	pt := ct.peers[rec.peer]
	if pt == nil {
		pt = &PeerConnTiming{}
		ct.peers[rec.peer] = pt
	}
	rec.stages[stage] = elapsed
	pt.Current.Stages = rec.stages
	pt.Current.LastStage = stage
	pt.Current.LastStageAt = elapsed
	pt.Current.SessionID = rec.sessionID
	pt.Current.Attempt = rec.attempt
	ct.mu.Unlock()
}

func (ct *connTiming) succeed(rec *connAttempt) {
	if rec == nil {
		return
	}
	ct.mu.Lock()
	pt := ct.peers[rec.peer]
	if pt != nil {
		pt.Current.CompletedAt = time.Now()
		pt.Current.AttemptFailed = false
		pt.LastSuccess = pt.Current
	}
	ct.mu.Unlock()
}

func (ct *connTiming) fail(rec *connAttempt, err string) {
	if rec == nil {
		return
	}
	ct.mu.Lock()
	pt := ct.peers[rec.peer]
	if pt != nil {
		pt.Current.AttemptFailed = true
		pt.Current.LastFailure = err
		pt.Current.CompletedAt = time.Now()
		pt.LastFailure = pt.Current
	}
	ct.mu.Unlock()
}

// NotePeerRecovery records transport recovery attempts for timing/diagnostics.
func (r *Runtime) NotePeerRecovery(peer string, started bool) {
	if r.connTiming != nil {
		r.connTiming.recovery(peer, started)
	}
}

func (ct *connTiming) recovery(peer string, started bool) {
	ct.mu.Lock()
	pt := ct.peers[peer]
	if pt == nil {
		pt = &PeerConnTiming{}
		ct.peers[peer] = pt
	}
	if started {
		pt.RecoveryCount++
	}
	ct.mu.Unlock()
}

// ConnStageOrder returns stages in typical establishment order for display.
func ConnStageOrder() []ConnStage {
	return []ConnStage{
		StageSignalingReady,
		StagePeerSessionCreated,
		StageRemoteCredReceived,
		StageGatherStarted,
		StageGatherCompleted,
		StageRemoteCandidate,
		StageICEConnectStarted,
		StageICEConnected,
		StageDataTransportReady,
		StageRecoveryStarted,
		StageRecoveryCompleted,
	}
}

func (ct *connTiming) snapshot(peer string) (PeerConnTiming, bool) {
	ct.mu.Lock()
	pt, ok := ct.peers[peer]
	if !ok || pt == nil {
		ct.mu.Unlock()
		return PeerConnTiming{}, false
	}
	out := *pt
	ct.mu.Unlock()
	return out, true
}
