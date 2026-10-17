package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/grantcostello/mynet/internal/config"
)

func TestStaleSocketRemoval(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".mynet")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := SocketPath(dir)
	if err := os.WriteFile(path, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RemoveStaleSocket(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("expected stale socket removed")
	}
}

func TestDaemonPing(t *testing.T) {
	dir := startTestDaemon(t)
	waitReachable(t, dir)
}

func TestDaemonPingAfterQuitCycle(t *testing.T) {
	dir := startTestDaemon(t)
	_ = RequestQuit(dir, DefaultQuitWait)
	waitNotReachable(t, dir)
	dir2 := startTestDaemon(t)
	waitReachable(t, dir2)
	_ = RequestQuit(dir2, DefaultQuitWait)
}

func testStateDir(t *testing.T) string {
	// macOS limits Unix socket paths (~104 bytes); t.TempDir() nested paths can exceed it.
	dir, err := os.MkdirTemp("/tmp", "mynetd")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func startTestDaemon(t *testing.T) string {
	dir := testStateDir(t)
	node := &Node{StateDir: dir}
	errCh := make(chan error, 1)
	go func() {
		errCh <- ServeIPC(dir, node)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if DaemonReachable(dir) {
			t.Cleanup(func() { _ = RequestQuit(dir, DefaultQuitWait) })
			return dir
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := drainServeErr(errCh); err != nil {
		t.Fatalf("daemon did not become reachable: %v", err)
	}
	t.Fatal("daemon did not become reachable")
	return dir
}

func drainServeErr(errCh <-chan error) error {
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

func TestDaemonQuit(t *testing.T) {
	dir := startTestDaemon(t)
	if err := RequestQuit(dir, DefaultQuitWait); err != nil {
		t.Fatal(err)
	}
	waitNotReachable(t, dir)
}

func TestRepeatedDownIdempotent(t *testing.T) {
	dir := startTestDaemon(t)
	for i := 0; i < 3; i++ {
		resp, err := CallIPC(dir, "down")
		if err != nil || !resp.OK {
			t.Fatalf("down %d: %v %+v", i, err, resp)
		}
	}
	_ = RequestQuit(dir, DefaultQuitWait)
}

func TestDownKeepsDaemonReachable(t *testing.T) {
	dir := startTestDaemon(t)
	resp, err := CallIPC(dir, "down")
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("down: %s", resp.Error)
	}
	if !DaemonReachable(dir) {
		t.Fatal("daemon should stay up after vpn down")
	}
	_ = RequestQuit(dir, DefaultQuitWait)
}

func TestProfileCPUOverIPCNoStateFile(t *testing.T) {
	dir := testStateDir(t)
	t.Setenv(envEnablePprof, "1")
	node := &Node{StateDir: dir}
	go func() { _ = ServeIPC(dir, node) }()
	waitReachable(t, dir)
	t.Cleanup(func() { _ = RequestQuit(dir, DefaultQuitWait) })

	resp, err := CallIPCRequest(dir, Request{Method: "profile_cpu", Seconds: 1})
	if err != nil || !resp.OK || resp.Profile == nil || resp.Profile.DataB64 == "" {
		t.Fatalf("profile_cpu: err=%v resp=%+v", err, resp)
	}
}

func waitNotReachable(t *testing.T, dir string) {
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !DaemonReachable(dir) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("daemon still reachable")
}

func waitReachable(t *testing.T, dir string) {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if DaemonReachable(dir) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("daemon ping timeout")
}

func TestResolveStateDirEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(config.EnvStateDir, dir)
	got, err := config.ResolveStateDir()
	if err != nil || got != dir {
		t.Fatalf("got %q err %v", got, err)
	}
}
