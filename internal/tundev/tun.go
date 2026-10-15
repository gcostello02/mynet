package tundev

import (
	"fmt"
	"net/netip"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"golang.zx2c4.com/wireguard/tun"
)

var routeMu sync.Mutex
var installedRoutes []string

// Open creates a TUN interface (requires elevated privileges on real hosts).
func Open(name string) (tun.Device, string, error) {
	if name == "" {
		name = "mynet0"
	}
	dev, err := tun.CreateTUN(name, 1420)
	if err != nil {
		return nil, "", err
	}
	actual, err := dev.Name()
	if err != nil {
		_ = dev.Close()
		return nil, "", err
	}
	return dev, actual, nil
}

// Configure assigns the local /32 address to the interface.
func Configure(ifName string, prefix netip.Prefix) error {
	switch runtime.GOOS {
	case "linux":
		return run("ip", "addr", "replace", prefix.String(), "dev", ifName)
	case "darwin":
		return run("ifconfig", ifName, "inet", prefix.Addr().String(), prefix.Addr().String())
	default:
		return fmt.Errorf("unsupported GOOS %s", runtime.GOOS)
	}
}

// AddPeerRoute installs a narrow /32 route for a peer via the TUN interface.
func AddPeerRoute(peer netip.Addr, ifName string) error {
	if err := RouteConflict(peer); err != nil {
		return err
	}
	routeMu.Lock()
	defer routeMu.Unlock()
	key := peer.String()
	for _, r := range installedRoutes {
		if r == key {
			return nil
		}
	}
	if err := addHostRoute(peer, ifName); err != nil {
		return err
	}
	installedRoutes = append(installedRoutes, key)
	return nil
}

func cleanupPeerRoute(peer netip.Addr, ifName string) error {
	routeMu.Lock()
	defer routeMu.Unlock()
	key := peer.String()
	switch runtime.GOOS {
	case "linux":
		_ = run("ip", "route", "del", key+"/32", "dev", ifName)
	case "darwin":
		_ = run("route", "-n", "delete", "-host", key)
	}
	out := make([]string, 0, len(installedRoutes))
	for _, r := range installedRoutes {
		if r != key {
			out = append(out, r)
		}
	}
	installedRoutes = out
	return nil
}

func addHostRoute(peer netip.Addr, ifName string) error {
	switch runtime.GOOS {
	case "linux":
		return run("ip", "route", "replace", peer.String()+"/32", "dev", ifName)
	case "darwin":
		return run("route", "-n", "add", "-host", peer.String(), "-interface", ifName)
	default:
		return fmt.Errorf("unsupported GOOS %s", runtime.GOOS)
	}
}

// CleanupRoutes removes routes installed by AddPeerRoute.
func CleanupRoutes(ifName string) error {
	routeMu.Lock()
	defer routeMu.Unlock()
	for _, peer := range installedRoutes {
		switch runtime.GOOS {
		case "linux":
			_ = run("ip", "route", "del", peer+"/32", "dev", ifName)
		case "darwin":
			_ = run("route", "-n", "delete", "-host", peer)
		}
	}
	installedRoutes = nil
	return nil
}

// RouteConflict reports explicit routing-table conflicts for peer/32 installation.
func RouteConflict(peer netip.Addr) error {
	switch runtime.GOOS {
	case "darwin":
		return darwinRouteConflict(peer)
	case "linux":
		return linuxRouteConflict(peer)
	default:
		return nil
	}
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v: %w (%s)", name, args, err, strings.TrimSpace(string(out)))
	}
	return nil
}
