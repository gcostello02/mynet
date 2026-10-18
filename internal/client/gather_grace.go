package client

import (
	"errors"
	"os"
	"strconv"
	"time"
)

// peerGatherGrace returns how long to wait for peer trickle/gather-done after local gathering completes.
func peerGatherGrace() time.Duration {
	const defaultGrace = 100 * time.Millisecond
	v := os.Getenv("MYNET_ICE_PEER_GATHER_GRACE_MS")
	if v == "" {
		return defaultGrace
	}
	ms, err := strconv.Atoi(v)
	if err != nil || ms < 0 {
		return defaultGrace
	}
	return time.Duration(ms) * time.Millisecond
}

// waitGatherComplete waits for local gathering, then for peer gather-done or a short grace window.
func waitGatherComplete(sig *signalState, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		sig.mu.Lock()
		local := sig.gatherDone
		sig.mu.Unlock()
		if local {
			return waitPeerGatherGrace(sig, peerGatherGrace())
		}
		time.Sleep(20 * time.Millisecond)
	}
	return errors.New("local ICE gathering did not complete")
}

func waitPeerGatherGrace(sig *signalState, grace time.Duration) error {
	if grace <= 0 {
		sig.mu.Lock()
		done := sig.peerGatherDone
		sig.mu.Unlock()
		if done {
			return nil
		}
		return nil
	}
	graceEnd := time.Now().Add(grace)
	for time.Now().Before(graceEnd) {
		sig.mu.Lock()
		done := sig.peerGatherDone
		sig.mu.Unlock()
		if done {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}
