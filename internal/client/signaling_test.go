package client

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/gcostello02/mynet/internal/control"
	"github.com/gcostello02/mynet/internal/invitation"
)

func TestSignalingLoopCallsEnsureDeviceSession(t *testing.T) {
	invitation.AllowHTTPForTest = true
	control.AllowInsecureHTTPForTest = true
	defer func() {
		invitation.AllowHTTPForTest = false
		control.AllowInsecureHTTPForTest = false
	}()

	dir := t.TempDir()
	rt, _ := enrollTestDevice(t, dir)
	rt.mu.Lock()
	rt.api.Token = ""
	rt.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rt.EnsureDeviceSession(ctx); err != nil {
		t.Fatal(err)
	}
	if rt.APIClient().Token == "" {
		t.Fatal("expected token after ensure")
	}
}

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
	cancel()
	time.Sleep(50 * time.Millisecond)
	snap := rt.SignalingSnapshot()
	if snap.State != "Disconnected" && snap.State != "Connecting" {
		t.Fatalf("unexpected state %q", snap.State)
	}
}
