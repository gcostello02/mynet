package client

import (
	"testing"
	"time"
)

func TestRecoveryTimelineCountsAttemptsAndWallClock(t *testing.T) {
	rt := newRecoveryTracker()
	rt.beginCycle("peer")
	rt.noteAttempt("peer")
	rt.noteFailure("peer", "credentials timeout")
	rt.noteAttempt("peer")
	dur := 120 * time.Millisecond
	time.Sleep(dur)
	rt.noteICESuccess("peer", 2, dur)
	rt.noteVPNReady("peer")

	snap, ok := rt.snapshot("peer")
	if !ok {
		t.Fatal("missing snapshot")
	}
	if snap.ICEAttemptsTotal != 2 {
		t.Fatalf("attempts total %d", snap.ICEAttemptsTotal)
	}
	if snap.ICEAttemptsFailed != 1 {
		t.Fatalf("failed %d", snap.ICEAttemptsFailed)
	}
	if snap.SuccessfulAttempt != 2 {
		t.Fatalf("success attempt %d", snap.SuccessfulAttempt)
	}
	if snap.TotalToICETransport < dur {
		t.Fatalf("total to ICE %v", snap.TotalToICETransport)
	}
	if snap.TotalToVPNReady <= 0 {
		t.Fatalf("expected VPN ready timing")
	}
}

func TestTransportRecoverySeparateFromICEFailures(t *testing.T) {
	rt := newRecoveryTracker()
	rt.noteTransportRecovery("peer")
	snap, _ := rt.snapshot("peer")
	if snap.TransportRecoveries != 1 {
		t.Fatalf("transport recoveries %d", snap.TransportRecoveries)
	}
	if snap.ICEAttemptsFailed != 0 {
		t.Fatalf("ice failures %d", snap.ICEAttemptsFailed)
	}
}
