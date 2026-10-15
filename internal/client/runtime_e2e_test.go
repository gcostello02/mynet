package client

import (
	"bytes"
	"context"
	"log/slog"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/grantcostello/mynet/internal/config"
	"github.com/grantcostello/mynet/internal/control/server"
)

func TestRuntimeP2POverServer(t *testing.T) {
	srv, err := server.New(server.Config{
		ListenAddr:       ":0",
		ServerSecret:     bytes.Repeat([]byte("s"), 32),
		EnrollmentSecret: "e2e-enroll",
		NetworkID:        "e2e",
	})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	dirA := t.TempDir()
	dirB := t.TempDir()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	SetICETraceHook(func(stage string, detail map[string]string) {
		t.Logf("ice stage=%s %v", stage, detail)
	})
	defer SetICETraceHook(nil)

	rtA := NewRuntime(dirA, log)
	rtB := NewRuntime(dirB, log)
	if err := rtA.Init("alpha"); err != nil {
		t.Fatal(err)
	}
	if err := rtB.Init("bravo"); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{dirA, dirB} {
		cfg, err := config.LoadClientConfig(d)
		if err != nil {
			t.Fatal(err)
		}
		cfg.ServerURL = ts.URL
		cfg.STUNURLs = nil
		cfg.TURNURLs = nil
		cfg.ICEURLs = nil
		cfg.Network.IndependentMode = true
		cfg.Network.ExcludeInterfaces = []string{"tailscale0", "utun*", "docker*", "br-*"}
		if err := config.SaveClientConfig(d, cfg); err != nil {
			t.Fatal(err)
		}
	}

	if err := rtA.Login("e2e-enroll"); err != nil {
		t.Fatal(err)
	}
	if err := rtB.Login("e2e-enroll"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rtA.StartSignaling(ctx)
	rtB.StartSignaling(ctx)

	var sig sync.WaitGroup
	sig.Add(2)
	go func() {
		defer sig.Done()
		if err := rtA.WaitForSignaling(context.Background(), 10*time.Second); err != nil {
			t.Error("alpha signaling:", err)
		}
	}()
	go func() {
		defer sig.Done()
		if err := rtB.WaitForSignaling(context.Background(), 10*time.Second); err != nil {
			t.Error("bravo signaling:", err)
		}
	}()
	sig.Wait()

	if err := rtA.Connect("bravo"); err != nil {
		t.Fatal(err)
	}
	rtt, err := rtA.Ping("bravo")
	if err != nil {
		t.Fatal(err)
	}
	if rtt <= 0 {
		t.Fatal("expected rtt")
	}
}

func TestHandleIncomingSessionSingleFlight(t *testing.T) {
	rt := NewRuntime(t.TempDir(), slog.Default())
	rt.incomingICE = map[string]bool{}
	sid := "sess-1"
	ready := struct {
		id string
	}{id: sid}

	var started int
	var mu sync.Mutex
	try := func() bool {
		rt.mu.Lock()
		defer rt.mu.Unlock()
		if rt.incomingICE[sid] {
			return false
		}
		rt.incomingICE[sid] = true
		mu.Lock()
		started++
		mu.Unlock()
		return true
	}
	if !try() {
		t.Fatal("first should enter")
	}
	if try() {
		t.Fatal("second should be rejected")
	}
	delete(rt.incomingICE, sid)
	if !try() {
		t.Fatal("third should enter after cleanup")
	}
	_ = ready
}
