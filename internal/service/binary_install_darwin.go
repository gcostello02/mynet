//go:build darwin

package service

import (
	"fmt"
	"os"
	"path/filepath"
)

// InstallDarwinSystemBinary copies source into DarwinSystemBinaryPath with root ownership.
// Caller must be root. Does not start launchd.
func InstallDarwinSystemBinary(source string) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("system binary install requires root")
	}
	if err := ValidateBinarySource(source); err != nil {
		return fmt.Errorf("candidate binary: %w", err)
	}
	dest := DarwinSystemBinaryPath
	if err := os.MkdirAll(DarwinSystemBinaryDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", DarwinSystemBinaryDir, err)
	}
	if err := chownRootWheel(DarwinSystemBinaryDir); err != nil {
		return fmt.Errorf("chown %s: %w", DarwinSystemBinaryDir, err)
	}
	if err := copyExecutableRoot(source, dest); err != nil {
		return err
	}
	if err := ValidateSupervisedBinaryTarget(dest); err != nil {
		return fmt.Errorf("installed binary failed security checks: %w", err)
	}
	return nil
}

func copyExecutableRoot(src, dst string) error {
	if err := validateNoSymlinkInPath(dst); err != nil {
		return err
	}
	b, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	dir := filepath.Dir(dst)
	tmp := filepath.Join(dir, "."+filepath.Base(dst)+".tmp")
	if err := os.WriteFile(tmp, b, 0o755); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := chownRootWheel(tmp); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("install %s: %w", dst, err)
	}
	return chownRootWheel(dst)
}

func chownRootWheel(path string) error {
	return os.Chown(path, 0, 0)
}

// FinalizeDarwinSystemBinary sets root ownership on an installed system binary (after upgrade swap).
func FinalizeDarwinSystemBinary(path string) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("requires root")
	}
	if path != DarwinSystemBinaryPath {
		return fmt.Errorf("refusing to finalize unexpected path %s", path)
	}
	if err := chownRootWheel(path); err != nil {
		return err
	}
	return ValidateSupervisedBinaryTarget(path)
}
