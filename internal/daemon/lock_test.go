package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDaemonLockExclusive(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".mynet")
	_ = os.MkdirAll(dir, 0o700)
	a, err := acquireDaemonLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = acquireDaemonLock(dir)
	if err == nil {
		t.Fatal("expected second lock to fail")
	}
	releaseDaemonLock(a)
	b, err := acquireDaemonLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	releaseDaemonLock(b)
}
