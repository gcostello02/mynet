package daemon

import (
	"context"
	"testing"
)

func TestNodeDownDoesNotCancelSignaling(t *testing.T) {
	n := &Node{StateDir: t.TempDir()}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	n.signalingCancel = cancel
	n.running = true

	if err := n.Down(); err != nil {
		t.Fatal(err)
	}
	if n.signalingCancel == nil {
		t.Fatal("Down cleared signalingCancel")
	}
	select {
	case <-ctx.Done():
		t.Fatal("Down cancelled signaling context")
	default:
	}
}
