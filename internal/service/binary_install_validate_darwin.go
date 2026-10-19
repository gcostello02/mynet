//go:build darwin

package service

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// validateDarwinLibexecInstallTree checks ownership and permissions on the libexec install path.
func validateDarwinLibexecInstallTree() error {
	dir := filepath.Clean(DarwinSystemBinaryDir)
	for i := 0; i < 4 && dir != "/" && dir != "."; i++ {
		fi, err := os.Lstat(dir)
		if err != nil {
			return fmt.Errorf("stat %s: %w", dir, err)
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("install directory must not be a symlink: %s", dir)
		}
		mode := fi.Mode().Perm()
		if mode&0o002 != 0 {
			return fmt.Errorf("install directory is world-writable: %s (%#o)", dir, mode)
		}
		if st, ok := fi.Sys().(*syscall.Stat_t); ok && st.Uid != 0 {
			return fmt.Errorf("install directory must be root-owned: %s (uid %d)", dir, st.Uid)
		}
		dir = filepath.Dir(dir)
	}
	return nil
}
