package service

import (
	"os"
	"path/filepath"
	"runtime"
)

// Label is the stable launchd/systemd service identifier.
const Label = "com.mynet.daemon"

// SystemLayout holds OS-specific install paths (overridable in tests via MYNET_SERVICE_ROOT).
type SystemLayout struct {
	LaunchDaemonPlist string
	LaunchDaemonEnv   string
	LogDir            string
	SystemdUnit       string
	SystemdEnvFile    string
	SystemdEnvDir     string
	LocalSpec         string
}

func SystemLayoutForOS() SystemLayout {
	root := os.Getenv("MYNET_SERVICE_ROOT")
	if root != "" {
		return testLayout(root)
	}
	switch runtime.GOOS {
	case "darwin":
		return SystemLayout{
			LaunchDaemonPlist: "/Library/LaunchDaemons/" + Label + ".plist",
			LaunchDaemonEnv:   "/Library/Preferences/" + Label + ".env",
			LogDir:            "/var/log/mynet",
			LocalSpec:         filepath.Join(os.Getenv("HOME"), ".mynet", "service.json"),
		}
	case "linux":
		return SystemLayout{
			SystemdUnit:    "/etc/systemd/system/mynet-daemon.service",
			SystemdEnvFile: "/etc/mynet/client.env",
			SystemdEnvDir:  "/etc/mynet",
			LogDir:         "/var/log/mynet",
			LocalSpec:      filepath.Join(os.Getenv("HOME"), ".mynet", "service.json"),
		}
	default:
		return SystemLayout{
			LocalSpec: filepath.Join(os.Getenv("HOME"), ".mynet", "service.json"),
		}
	}
}

func testLayout(root string) SystemLayout {
	return SystemLayout{
		LaunchDaemonPlist: filepath.Join(root, "Library/LaunchDaemons", Label+".plist"),
		LaunchDaemonEnv:   filepath.Join(root, "Library/Preferences", Label+".env"),
		SystemdUnit:       filepath.Join(root, "etc/systemd/system/mynet-daemon.service"),
		SystemdEnvFile:    filepath.Join(root, "etc/mynet/client.env"),
		SystemdEnvDir:     filepath.Join(root, "etc/mynet"),
		LogDir:            filepath.Join(root, "var/log/mynet"),
		LocalSpec:         filepath.Join(root, "local/service.json"),
	}
}
