package service

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// Start asks the native supervisor to start the daemon.
func Start() error {
	if osGeteuid() != 0 {
		return fmt.Errorf("service start requires root (sudo mynet service start)")
	}
	switch runtime.GOOS {
	case "linux":
		return runCmd("systemctl", "start", "mynet-daemon")
	case "darwin":
		layout := SystemLayoutForOS()
		return runCmd("launchctl", "bootstrap", "system", layout.LaunchDaemonPlist)
	default:
		return fmt.Errorf("unsupported OS")
	}
}

// Stop asks the supervisor to stop the daemon gracefully.
func Stop() error {
	if osGeteuid() != 0 {
		return fmt.Errorf("service stop requires root (sudo mynet service stop)")
	}
	switch runtime.GOOS {
	case "linux":
		return runCmd("systemctl", "stop", "mynet-daemon")
	case "darwin":
		layout := SystemLayoutForOS()
		return runCmd("launchctl", "bootout", "system", layout.LaunchDaemonPlist)
	default:
		return fmt.Errorf("unsupported OS")
	}
}

// Restart reloads the supervised daemon.
func Restart() error {
	if osGeteuid() != 0 {
		return fmt.Errorf("service restart requires root (sudo mynet service restart)")
	}
	switch runtime.GOOS {
	case "linux":
		return runCmd("systemctl", "restart", "mynet-daemon")
	case "darwin":
		if err := Stop(); err != nil {
			return err
		}
		return Start()
	default:
		return fmt.Errorf("unsupported OS")
	}
}

// Logs tails platform logs (best effort).
func Logs(follow bool) error {
	switch runtime.GOOS {
	case "linux":
		if follow {
			return exec.Command("journalctl", "-u", "mynet-daemon", "-f").Run()
		}
		return exec.Command("journalctl", "-u", "mynet-daemon", "-n", "100", "--no-pager").Run()
	case "darwin":
		layout := SystemLayoutForOS()
		path := layout.LogDir + "/daemon.log"
		if follow {
			return runCmd("tail", "-f", path)
		}
		return runCmd("tail", "-n", "100", path)
	default:
		return fmt.Errorf("unsupported OS")
	}
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %v: %w", name, args, err)
	}
	return nil
}

func osGeteuid() int {
	return os.Geteuid()
}
