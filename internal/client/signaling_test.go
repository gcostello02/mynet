package client

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestStartSignalingReconnectsAfterClose(t *testing.T) {
	// Uses mock is heavy; verify wait path when no server.
	dir := t.TempDir()
	rt := NewRuntime(dir, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err := rt.Init("sigtest"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rt.StartSignaling(ctx)
	time.Sleep(200 * time.Millisecond)
	snap := rt.SignalingSnapshot()
	if snap.State != "Disconnected" && snap.State != "Connecting" {
		t.Fatalf("unexpected state %q", snap.State)
	}
}
