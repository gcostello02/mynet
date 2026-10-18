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
