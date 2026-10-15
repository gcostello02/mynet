package tundev

import (
	"fmt"
	"net"
	"net/netip"
	"os/exec"
	"strings"
)

func darwinRouteConflict(peer netip.Addr) error {
	if !peer.Is4() {
		return nil
	}
	out, err := exec.Command("netstat", "-rn", "-f", "inet").Output()
	if err != nil {
		return nil
	}
	conflicts := darwinConflicts(peer, string(out))
	if len(conflicts) == 0 {
		return nil
	}
	return fmt.Errorf("conflicting route for %s: %s", peer, strings.Join(conflicts, "; "))
}

func darwinConflicts(peer netip.Addr, netstat string) []string {
	var conflicts []string
	peerStr := peer.String()
	for _, line := range strings.Split(netstat, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Routing") || strings.HasPrefix(line, "Internet") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		dest := fields[0]
		if dest == "default" || dest == "0.0.0.0" {
			continue
		}
		gateway := fields[1]
		flags := ""
		netif := ""
		if len(fields) >= 3 {
			flags = fields[2]
		}
		if len(fields) >= 4 {
			netif = fields[3]
		}
		if conflictDest(dest, gateway, flags, netif, peerStr, peer) {
			conflicts = append(conflicts, line)
		}
	}
	return conflicts
}

func conflictDest(dest, gateway, flags, netif, peerStr string, peer netip.Addr) bool {
	if pfx, ok := parseDarwinRouteDest(dest); ok {
		if pfx.Contains(peer) && pfx.Bits() < 32 {
			if isPhysicalDarwinIface(netif) {
				return true
			}
		}
		if pfx.Addr() == peer && pfx.Bits() == 32 {
			if isPhysicalDarwinIface(netif) {
				return true
			}
		}
	}
	if dest == peerStr {
		if isTransientDarwinHost(flags, gateway, netif) {
			return false
		}
		if isMynetDarwinIface(netif) {
			return false
		}
		return true
	}
	// Avoid matching 10.77.0.10 when looking for 10.77.0.1
	if strings.HasPrefix(dest, peerStr) && len(dest) > len(peerStr) {
		next := dest[len(peerStr)]
		if next == '.' || next == '/' {
			return false
		}
	}
	return false
}

func parseDarwinRouteDest(dest string) (netip.Prefix, bool) {
	if !strings.Contains(dest, "/") {
		return netip.Prefix{}, false
	}
	parts := strings.Split(dest, "/")
	if len(parts) != 2 {
		return netip.Prefix{}, false
	}
	var bitsN int
	if _, err := fmt.Sscanf(parts[1], "%d", &bitsN); err != nil || bitsN < 0 || bitsN > 32 {
		return netip.Prefix{}, false
	}
	ipPart := parts[0]
	if !strings.Contains(ipPart, ".") {
		ipPart += ".0.0.0"
	} else {
		// Normalize short forms like 10.0.0/22
		ip := net.ParseIP(ipPart)
		if ip == nil {
			return netip.Prefix{}, false
		}
		ipPart = ip.String()
	}
	cidr := ipPart + "/" + parts[1]
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		return netip.Prefix{}, false
	}
	pfx, err := netip.ParsePrefix(n.String())
	return pfx, err == nil
}

func isPhysicalDarwinIface(netif string) bool {
	if netif == "" {
		return false
	}
	if isMynetDarwinIface(netif) {
		return false
	}
	switch {
	case strings.HasPrefix(netif, "en"),
		strings.HasPrefix(netif, "bridge"),
		strings.HasPrefix(netif, "awdl"),
		strings.HasPrefix(netif, "llw"):
		return true
	default:
		return false
	}
}

func isMynetDarwinIface(netif string) bool {
	return netif == "mynet0" || strings.HasPrefix(netif, "utun")
}

// Transient host entries (neighbor/ARP cache) use link# or MAC gateways with UHLWI* flags.
func isTransientDarwinHost(flags, gateway, netif string) bool {
	if strings.Contains(flags, "UHLW") {
		return true
	}
	if isPhysicalDarwinIface(netif) && strings.Contains(flags, "L") {
		if strings.HasPrefix(gateway, "link#") || strings.Contains(gateway, ":") {
			return true
		}
	}
	return false
}
