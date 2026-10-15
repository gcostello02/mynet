package client

import (
	"bytes"
	"log/slog"
	"net/http/httptest"
	"os"
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
	if err := rtB.ensureWebSocket(); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			time.Sleep(time.Second)
		}
	}()

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
