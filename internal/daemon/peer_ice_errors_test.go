package daemon

import (
	"context"
	"errors"
	"testing"
)

func TestPeerICEErrorCountsAsFailure(t *testing.T) {
	if peerICEErrorCountsAsFailure(nil) {
		t.Fatal("nil")
	}
	if !peerICEErrorCountsAsFailure(context.DeadlineExceeded) {
		t.Fatal("deadline")
	}
	if !peerICEErrorCountsAsFailure(errors.New("timed out waiting for peer ICE credentials")) {
		t.Fatal("credentials")
	}
	if peerICEErrorCountsAsFailure(errors.New("canceled")) {
		t.Fatal("benign")
	}
}
