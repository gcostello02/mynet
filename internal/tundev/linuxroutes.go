package tundev

import (
	"fmt"
	"net/netip"
	"os/exec"
	"strings"
)

// linuxRouteConflict inspects the routing tables for explicit routes that would
// conflict with installing peer/32 via MyNet TUN. Default-route resolution from
// `ip route get` is intentionally not used (it always matches via default).
func linuxRouteConflict(peer netip.Addr) error {
	if !peer.Is4() {
		return nil
	}
	if err := linuxLocalAddrConflict(peer); err != nil {
		return err
	}
	out, err := exec.Command("ip", "-4", "route", "show", "table", "all").Output()
	if err != nil {
		return fmt.Errorf("could not inspect routes (ip -4 route show table all): %w", err)
	}
	routes := parseLinuxRoutes(string(out))
	conflicts := explicitConflicts(peer, routes)
	if len(conflicts) == 0 {
		return nil
	}
	if len(conflicts) == 1 {
		return fmt.Errorf("conflicting route for %s: %s", peer, conflicts[0])
	}
	return fmt.Errorf("conflicting routes for %s: %s", peer, strings.Join(conflicts, "; "))
}

func linuxLocalAddrConflict(peer netip.Addr) error {
	out, err := exec.Command("ip", "-4", "addr", "show").Output()
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "inet ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pfx, err := netip.ParsePrefix(fields[1])
		if err != nil {
			continue
		}
		if pfx.Contains(peer) && pfx.Addr() == peer {
			return fmt.Errorf("address %s is already assigned on a local interface (%s)", peer, strings.TrimPrefix(fields[len(fields)-1], " "))
		}
	}
	return nil
}

type linuxRoute struct {
	raw   string
	dst   netip.Prefix
	dev   string
	typ   string // unreachable, blackhole, throw, etc.
	table string
}

func parseLinuxRoutes(output string) []linuxRoute {
	var routes []linuxRoute
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		r, ok := parseLinuxRouteLine(line)
		if ok {
			routes = append(routes, r)
		}
	}
	return routes
}

func parseLinuxRouteLine(line string) (linuxRoute, bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return linuxRoute{}, false
	}
	r := linuxRoute{raw: line}
	for i, f := range fields {
		switch f {
		case "table":
			if i+1 < len(fields) {
				r.table = fields[i+1]
			}
		case "dev":
			if i+1 < len(fields) {
				r.dev = fields[i+1]
			}
		case "unreachable", "blackhole", "prohibit", "throw":
			r.typ = f
		}
	}
	dstTok := fields[0]
	if dstTok == "default" {
		r.dst = netip.PrefixFrom(netip.IPv4Unspecified(), 0)
		return r, true
	}
	if strings.Contains(dstTok, "/") {
		pfx, err := netip.ParsePrefix(dstTok)
		if err != nil {
			return linuxRoute{}, false
		}
		r.dst = pfx
		return r, true
	}
	addr, err := netip.ParseAddr(dstTok)
	if err != nil {
		return linuxRoute{}, false
	}
	r.dst = netip.PrefixFrom(addr, addr.BitLen())
	return r, true
}

func explicitConflicts(peer netip.Addr, routes []linuxRoute) []string {
	var out []string
	for _, r := range routes {
		if !routeCoversPeer(r, peer) {
			continue
		}
		if isBenignLinuxRoute(r, peer) {
			continue
		}
		if r.dst.Bits() == 0 {
			continue // default route — normal FIB fallback, not an explicit peer route
		}
		out = append(out, strings.TrimSpace(r.raw))
	}
	return out
}

func routeCoversPeer(r linuxRoute, peer netip.Addr) bool {
	if !r.dst.IsValid() {
		return false
	}
	if r.dst.Contains(peer) {
		return true
	}
	// Host route written as "10.77.0.1" without /32
	if r.dst.Bits() == peer.BitLen() && r.dst.Addr() == peer {
		return true
	}
	return false
}

func isBenignLinuxRoute(r linuxRoute, peer netip.Addr) bool {
	// Local / broadcast / multicast noise in `table local`
	if r.table == "local" {
		if r.dst.Addr() == peer && r.dst.Bits() == peer.BitLen() {
			// kernel local route for an address we own on lo or another iface is caught by addr show
			return true
		}
		return true
	}
	if r.table == "main" && r.dev == "lo" {
		return true
	}
	return false
}
