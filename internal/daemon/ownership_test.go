package daemon

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/gcostello02/mynet/internal/config"
)

func TestChownToConfiguredOwnerSkipsNonRoot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.EnvOwnerUser, "nobody")
	if err := ChownToConfiguredOwner(path); err != nil {
		t.Fatal(err)
	}
}

func TestRefuseSymlinkForChown(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "real")
	if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := refuseSymlinkForChown(link); err == nil {
		t.Fatal("expected symlink rejection")
	}
}

func TestChownSupervisedRequiresOwner(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("root required")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "sock")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.EnvSupervised, "1")
	t.Setenv(config.EnvOwnerUser, "")
	if err := ChownToConfiguredOwner(path); err == nil {
		t.Fatal("expected error without owner when supervised")
	}
}

func TestChownToConfiguredOwnerAsRootSetsOwner(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("root required")
	}
	me, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "mynetd.sock")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.EnvOwnerUser, me.Username)
	if err := ChownToConfiguredOwner(path); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("mode: %o", st.Mode())
	}
}

func TestWritePIDFileOwnershipAsRoot(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("root required")
	}
	me, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	t.Setenv(config.EnvOwnerUser, me.Username)
	if err := WritePIDFile(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(PIDPath(dir)); err != nil {
		t.Fatal(err)
	}
}

func TestServeIPCChownBeforeAccept(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("root required")
	}
	me, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	dir := testStateDir(t)
	t.Setenv(config.EnvOwnerUser, me.Username)
	node := &Node{StateDir: dir}
	errCh := make(chan error, 1)
	go func() {
		errCh <- ServeIPC(dir, node)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !DaemonReachable(dir) {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		st, err := os.Stat(SocketPath(dir))
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm() != 0o600 {
			t.Fatalf("socket mode: %o", st.Mode().Perm())
		}
		t.Cleanup(func() { _ = RequestQuit(dir, DefaultQuitWait) })
		return
	}
	if err := drainServeErr(errCh); err != nil {
		t.Fatalf("serve: %v", err)
	}
	t.Fatal("daemon did not become reachable")
}

func TestWritePIDFileDoesNotTouchIdentity(t *testing.T) {
	dir := t.TempDir()
	idPath := filepath.Join(dir, "identity.json")
	if err := os.WriteFile(idPath, []byte(`{"device_id":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(idPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := WritePIDFile(dir); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(idPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("identity.json changed")
	}
}
