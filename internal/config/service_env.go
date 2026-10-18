package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnvSupervised is set by launchd/systemd when the daemon is service-managed.
const EnvSupervised = "MYNET_SUPERVISED"

// EnvRunVPNAtStart brings the VPN data plane up when the daemon starts (always-on).
const EnvRunVPNAtStart = "MYNET_RUN_VPN_AT_START"

// EnvOwnerUser names the human user whose ~/.mynet identity is used when the daemon runs as root.
const EnvOwnerUser = "MYNET_OWNER_USER"

// EnvServiceBinary overrides the mynet binary path recorded in service definitions.
const EnvServiceBinary = "MYNET_SERVICE_BINARY"

func Supervised() bool {
	return strings.TrimSpace(os.Getenv(EnvSupervised)) == "1"
}

func RunVPNAtStart() bool {
	return strings.TrimSpace(os.Getenv(EnvRunVPNAtStart)) == "1"
}

// ValidateStateDirPath rejects unsafe state directory paths for service install.
func ValidateStateDirPath(dir string) error {
	if dir == "" {
		return fmt.Errorf("empty state directory")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(abs) {
		return fmt.Errorf("state directory must be absolute: %s", dir)
	}
	clean := filepath.Clean(abs)
	if strings.Contains(clean, "..") {
		return fmt.Errorf("invalid state directory")
	}
	fi, err := os.Stat(clean)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("state directory does not exist: %s (run mynet init/login first)", clean)
		}
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("state path is not a directory: %s", clean)
	}
	return nil
}

// OwnerUsername returns the configured owner for root-owned daemons.
func OwnerUsername() string {
	if u := strings.TrimSpace(os.Getenv(EnvOwnerUser)); u != "" {
		return u
	}
	return strings.TrimSpace(os.Getenv("SUDO_USER"))
}
