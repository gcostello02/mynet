package service

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/grantcostello/mynet/internal/config"
	"github.com/grantcostello/mynet/internal/daemon"
)

// PreflightInstall ensures it is safe to register a supervisor without competing daemons.
func PreflightInstall(stateDir, binaryPath string) error {
	if err := config.ValidateStateDirPath(stateDir); err != nil {
		return err
	}
	if err := config.IdentityMustExist(stateDir); err != nil {
		return err
	}
	if binaryPath == "" {
		return fmt.Errorf("binary path required")
	}
	abs, err := filepath.Abs(binaryPath)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(abs) {
		return fmt.Errorf("binary path must be absolute: %s", binaryPath)
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return fmt.Errorf("binary %s: %w", abs, err)
	}
	if fi.IsDir() {
		return fmt.Errorf("binary path is a directory: %s", abs)
	}
	if daemon.DaemonReachable(stateDir) {
		return fmt.Errorf("mynet daemon still running for %s — stop manual VPN/daemon before service install (mynet daemon stop; confirm with mynet status)", stateDir)
	}
	if daemon.IsProcessAlive(stateDir) {
		return fmt.Errorf("mynet pid file indicates a live process for %s — resolve before service install", stateDir)
	}
	_ = daemon.RemoveStaleSocket(stateDir)
	return nil
}

// EnsureDaemonStopped is required before binary swap during upgrade.
func EnsureDaemonStopped(stateDir string) error {
	if daemon.DaemonReachable(stateDir) {
		return fmt.Errorf("daemon still running for %s; stop service or daemon before binary replacement", stateDir)
	}
	if daemon.IsProcessAlive(stateDir) {
		return fmt.Errorf("pid file still live for %s", stateDir)
	}
	return nil
}
