package client

import "time"

// connErrorRecord tracks the most recent connectivity failure for diagnostics.
type connErrorRecord struct {
	Message   string
	At        time.Time
	Peer      string
	SessionID string
	Kind      string
}

func (r *Runtime) recordConnError(peer, sessionID, kind string, err error) {
	if err == nil {
		return
	}
	r.mu.Lock()
	msg := err.Error()
	r.lastErr = msg
	r.lastErrAt = time.Now()
	r.lastErrPeer = peer
	r.lastErrKind = kind
	r.lastErrSession = sessionID
	r.lastHistoricalErr = msg
	r.lastHistoricalErrAt = r.lastErrAt
	r.mu.Unlock()
}

func (r *Runtime) clearConnErrorForPeer(peer string) {
	r.mu.Lock()
	if r.lastErrPeer == peer || peer == "" {
		r.lastErr = ""
		r.lastErrKind = ""
		r.lastErrSession = ""
	}
	r.lastSuccessAt = time.Now()
	r.lastSuccessPeer = peer
	r.mu.Unlock()
}

// effectiveLastError returns a user-visible error only when it still applies to the peer.
func (r *Runtime) effectiveLastError(peerName string, dataPathReady bool) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if dataPathReady {
		return ""
	}
	if r.lastErr == "" {
		return ""
	}
	if peerName != "" && r.lastErrPeer != "" && r.lastErrPeer != peerName {
		return ""
	}
	return r.lastErr
}

// EffectiveLastError exposes last connectivity error when it still applies.
func (r *Runtime) EffectiveLastError(peerName string, dataPathReady bool) string {
	return r.effectiveLastError(peerName, dataPathReady)
}

func (r *Runtime) historicalLastError() (string, time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastHistoricalErr, r.lastHistoricalErrAt
}
