package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/grantcostello/mynet/internal/config"
)

func TestAcquireLockSupervisedUsesExistingStateDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "identity.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.EnvSupervised, "1")
	t.Setenv(config.EnvStateDir, dir)

	lock, err := acquireDaemonLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	releaseDaemonLock(lock)
}

func TestAcquireLockSupervisedRejectsMissingStateDir(t *testing.T) {
	t.Setenv(config.EnvSupervised, "1")
	missing := filepath.Join(t.TempDir(), "nope")
	_, err := acquireDaemonLock(missing)
	if err == nil {
		t.Fatal("expected error for missing supervised state dir")
	}
}
