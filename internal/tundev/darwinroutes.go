package tundev

import (
	"fmt"
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
		if conflictDest(dest, peerStr, peer) {
			conflicts = append(conflicts, line)
		}
	}
	return conflicts
}

func conflictDest(dest string, peerStr string, peer netip.Addr) bool {
	if dest == peerStr {
		return true
	}
	if strings.Contains(dest, "/") {
		pfx, err := netip.ParsePrefix(dest)
		if err == nil && pfx.Contains(peer) && pfx.Bits() < 32 {
			return true
		}
		if err == nil && pfx.Addr() == peer && pfx.Bits() == 32 {
			return true
		}
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
