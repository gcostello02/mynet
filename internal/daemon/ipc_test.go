package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gcostello02/mynet/internal/config"
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

func startTestDaemon(t *testing.T) string {
	dir := filepath.Join(t.TempDir(), ".mynet")
	_ = os.MkdirAll(dir, 0o700)
	node := &Node{StateDir: dir}
	go func() {
		_ = ServeIPC(dir, node)
	}()
	t.Cleanup(func() { _ = RequestQuit(dir) })
	return dir
}

func TestDaemonQuit(t *testing.T) {
	dir := startTestDaemon(t)
	waitReachable(t, dir)
	if err := RequestQuit(dir); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !DaemonReachable(dir) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("daemon still reachable after quit")
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
