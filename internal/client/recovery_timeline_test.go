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

func TestConnTimingResetsWithTransportRecovery(t *testing.T) {
	rt := NewRuntime(t.TempDir(), nil)
	rt.connTiming.beginAttempt("peer", "s1")
	rt.connTiming.beginAttempt("peer", "s2")
	rt.NoteTransportRecovery("peer")
	rec := rt.connTiming.beginAttempt("peer", "s3")
	if rec.attempt != 1 {
		t.Fatalf("expected attempt 1 after cycle reset, got %d", rec.attempt)
	}
}

func TestCoherentAttemptCountsAlignsStaleConnTiming(t *testing.T) {
	tl := RecoveryTimeline{ICEAttemptsTotal: 0, SuccessfulAttempt: 2}
	total, idx := tl.CoherentAttemptCounts()
	if total != 2 || idx != 2 {
		t.Fatalf("total=%d idx=%d", total, idx)
	}
}

func TestICESuccessAlignsAttemptCounters(t *testing.T) {
	tracker := newRecoveryTracker()
	tracker.beginCycle("peer")
	tracker.noteICESuccess("peer", 2, 100*time.Millisecond)
	snap, _ := tracker.snapshot("peer")
	total, idx := snap.CoherentAttemptCounts()
	if total != 2 || idx != 2 {
		t.Fatalf("total=%d idx=%d", total, idx)
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
