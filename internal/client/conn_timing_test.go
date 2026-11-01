package client

import (
	"testing"
	"time"
)

func TestConnTimingRecordsStagesInOrder(t *testing.T) {
	ct := newConnTiming()
	rec := ct.beginAttempt("peer", "sess-1")
	ct.mark(rec, StageSignalingReady)
	time.Sleep(2 * time.Millisecond)
	ct.mark(rec, StageICEConnected)
	ct.succeed(rec)

	snap, ok := ct.snapshot("peer")
	if !ok {
		t.Fatal("missing snapshot")
	}
	if snap.Current.Stages[StageSignalingReady] > snap.Current.Stages[StageICEConnected] {
		t.Fatal("expected monotonic stage times")
	}
	if snap.LastSuccess.SessionID != "sess-1" {
		t.Fatalf("last success session: %s", snap.LastSuccess.SessionID)
	}
}

func TestMarkExistingAfterCycleResetNotEpochDuration(t *testing.T) {
	ct := newConnTiming()
	rec := ct.beginAttempt("peer", "s1")
	ct.mark(rec, StageDataTransportReady)
	ct.resetPeerCycle("peer")
	ct.markExisting("peer", "s2", StageWireGuardHandshake)
	snap, ok := ct.snapshot("peer")
	if !ok {
		t.Fatal("missing snapshot")
	}
	wg, ok := snap.Current.Stages[StageWireGuardHandshake]
	if !ok {
		t.Fatal("expected wireguard stage")
	}
	if !ConnStageDurationOK(wg) {
		t.Fatalf("wireguard duration not displayable: %v", wg)
	}
	if wg > 5*time.Second {
		t.Fatalf("expected small post-reset duration, got %v", wg)
	}
}

func TestSanitizeRemovesAbsurdStageDurations(t *testing.T) {
	snap := ConnTimingSnapshot{
		Stages: map[ConnStage]time.Duration{
			StageWireGuardHandshake: maxDisplayConnStage + time.Hour,
			StageICEToWireGuard:     50 * time.Millisecond,
			StageDataTransportReady: 10 * time.Millisecond,
		},
		LastStageAt: 200 * 365 * 24 * time.Hour,
		LastStage:   StageWireGuardHandshake,
	}
	out := sanitizeTimingSnapshot(snap)
	if _, ok := out.Stages[StageWireGuardHandshake]; ok {
		t.Fatal("absurd duration should be stripped")
	}
	if out.Stages[StageICEToWireGuard] != 50*time.Millisecond {
		t.Fatalf("expected valid stage kept")
	}
}

func TestConnStageDurationOK(t *testing.T) {
	if ConnStageDurationOK(-1) {
		t.Fatal("negative")
	}
	if !ConnStageDurationOK(time.Second) {
		t.Fatal("one second ok")
	}
	if ConnStageDurationOK(maxDisplayConnStage + time.Hour) {
		t.Fatal("over max display not ok")
	}
}
