package service

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/grantcostello/mynet/internal/config"
	"github.com/grantcostello/mynet/internal/daemon"
)

// CompetingDaemonError is returned when a manual daemon would conflict with service management.
type CompetingDaemonError struct {
	StateDir string
	PIDHint  string
}

func (e *CompetingDaemonError) Error() string {
	return fmt.Sprintf("mynet daemon already running for state dir %s (use mynet service stop or mynet daemon stop first)", e.StateDir)
}

// Installed reports whether local service metadata exists.
func Installed(localSpecPath string) bool {
	_, ok, err := LoadLocalSpec(localSpecPath)
	return err == nil && ok
}

// SystemUnitInstalled checks for the platform unit file on disk.
func SystemUnitInstalled(layout SystemLayout) bool {
	switch runtime.GOOS {
	case "darwin":
		_, err := os.Stat(layout.LaunchDaemonPlist)
		return err == nil
	case "linux":
		_, err := os.Stat(layout.SystemdUnit)
		return err == nil
	default:
		return false
	}
}

// DaemonReachableForState reports IPC health for the given state directory.
func DaemonReachableForState(stateDir string) bool {
	return daemon.DaemonReachable(stateDir)
}

// ShouldSpawnDetachedDaemon returns false when a supervised install exists and expects the supervisor to own the process.
func ShouldSpawnDetachedDaemon(stateDir string, localSpecPath string) (bool, string) {
	if config.Supervised() {
		return false, "already running under service supervisor in this process"
	}
	spec, ok, err := LoadLocalSpec(localSpecPath)
	if err != nil || !ok {
		return true, ""
	}
	if spec.StateDir != "" && filepath.Clean(spec.StateDir) != filepath.Clean(stateDir) {
		return true, ""
	}
	if SystemUnitInstalled(SystemLayoutForOS()) && !daemon.DaemonReachable(stateDir) {
		return false, "service is installed but daemon is not reachable — run: mynet service start (or sudo systemctl/launchctl)"
	}
	return true, ""
}
