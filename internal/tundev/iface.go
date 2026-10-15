package tundev

import (
	"fmt"
	"net/netip"
	"runtime"
)

// activateInterface brings the TUN netdev administratively up (required on Linux before routes).
func activateInterface(ifName string) error {
	switch runtime.GOOS {
	case "linux":
		if err := run("ip", "link", "set", "dev", ifName, "up"); err != nil {
			return fmt.Errorf("ip link set up: %w", err)
		}
		if err := run("ip", "link", "set", "dev", ifName, "mtu", "1420"); err != nil {
			return fmt.Errorf("ip link set mtu: %w", err)
		}
		return nil
	case "darwin":
		if err := run("ifconfig", ifName, "up"); err != nil {
			return fmt.Errorf("ifconfig up: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported GOOS %s", runtime.GOOS)
	}
}

// startupSteps documents the required VPN interface initialization order (for tests).
var startupSteps = []string{
	"open_tun",
	"assign_address",
	"activate_interface",
	"wireguard_up",
	"peer_routes",
	"wireguard_peers",
	"ice_attach",
}

// StartupStepCount returns the number of ordered startup phases.
func StartupStepCount() int { return len(startupSteps) }

// ConfigureAndActivate assigns the local /32 and brings the interface up.
func ConfigureAndActivate(ifName string, prefix netip.Prefix) error {
	if err := Configure(ifName, prefix); err != nil {
		return fmt.Errorf("assign address %s on %s: %w", prefix, ifName, err)
	}
	if err := activateInterface(ifName); err != nil {
		return fmt.Errorf("activate %s: %w", ifName, err)
	}
	return nil
}
