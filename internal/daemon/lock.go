package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/gcostello02/mynet/internal/config"
)

func lockPath(stateDir string) string {
	return filepath.Join(stateDir, "mynetd.lock")
}

func prepareStateDir(stateDir string) error {
	if config.Supervised() {
		// Supervised daemons must use an existing user state tree; never create ~ or .mynet as root.
		return config.ValidateStateDirPath(stateDir)
	}
	return os.MkdirAll(stateDir, 0o700)
}

// acquireDaemonLock returns an open file holding an exclusive flock for this state dir.
func acquireDaemonLock(stateDir string) (*os.File, error) {
	if err := prepareStateDir(stateDir); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(lockPath(stateDir), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("another mynet daemon already holds %s", lockPath(stateDir))
	}
	return f, nil
}

func releaseDaemonLock(f *os.File) {
	if f == nil {
		return
	}
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	_ = f.Close()
}
