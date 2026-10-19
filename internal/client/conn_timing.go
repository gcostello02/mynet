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
	StageWireGuardHandshake   ConnStage = "wireguard_handshake"
	StageVPNDataUsable        ConnStage = "vpn_data_usable"
	StageICEToWireGuard       ConnStage = "ice_to_wireguard_handshake"
	StageICEToVPNReady        ConnStage = "ice_to_vpn_data_usable"
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

func (ct *connTiming) resetPeerCycle(peer string) {
	ct.mu.Lock()
	if pt, ok := ct.peers[peer]; ok && pt != nil {
		pt.Current = ConnTimingSnapshot{}
	}
	ct.mu.Unlock()
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

// MarkConnStage records a timing stage on the current attempt for a peer (e.g. WireGuard handshake).
func (r *Runtime) MarkConnStage(peer, sessionID string, stage ConnStage) {
	if r.connTiming == nil {
		return
	}
	r.connTiming.markExisting(peer, sessionID, stage)
}

func (ct *connTiming) markExisting(peer, sessionID string, stage ConnStage) {
	ct.mu.Lock()
	pt := ct.peers[peer]
	if pt == nil {
		ct.mu.Unlock()
		return
	}
	elapsed := time.Since(pt.Current.StartedAt)
	if pt.Current.Stages == nil {
		pt.Current.Stages = make(map[ConnStage]time.Duration)
	}
	pt.Current.Stages[stage] = elapsed
	pt.Current.LastStage = stage
	pt.Current.LastStageAt = elapsed
	if stage == StageWireGuardHandshake || stage == StageVPNDataUsable {
		if iceReady, ok := pt.Current.Stages[StageDataTransportReady]; ok {
			delta := elapsed - iceReady
			if delta < 0 {
				delta = 0
			}
			if stage == StageWireGuardHandshake {
				pt.Current.Stages[StageICEToWireGuard] = delta
			} else {
				pt.Current.Stages[StageICEToVPNReady] = delta
			}
		}
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
		StageICEToWireGuard,
		StageWireGuardHandshake,
		StageICEToVPNReady,
		StageVPNDataUsable,
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
