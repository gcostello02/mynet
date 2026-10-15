package tundev

import (
	"net/netip"
	"testing"
)

func TestExplicitConflictsDefaultRouteOnly(t *testing.T) {
	peer := netip.MustParseAddr("10.77.0.1")
	table := `
default via 10.0.0.1 dev enp1s0 proto dhcp src 10.0.0.50 metric 100
10.0.0.0/24 dev enp1s0 proto kernel scope link src 10.0.0.50
`
	routes := parseLinuxRoutes(table)
	conflicts := explicitConflicts(peer, routes)
	if len(conflicts) != 0 {
		t.Fatalf("expected no conflict with default only, got %v", conflicts)
	}
}

func TestExplicitConflictsDefaultGatewayFalsePositiveRegression(t *testing.T) {
	// Regression: `ip route get` reported this path; `ip route show table all` must not.
	peer := netip.MustParseAddr("10.77.0.1")
	table := `
default via 10.0.0.1 dev enp1s0 proto dhcp src 10.0.0.50 metric 100
`
	conflicts := explicitConflicts(peer, parseLinuxRoutes(table))
	if len(conflicts) != 0 {
		t.Fatalf("got %v", conflicts)
	}
}

func TestExplicitConflictsHostRoute(t *testing.T) {
	peer := netip.MustParseAddr("10.77.0.1")
	table := `
10.77.0.1 dev wg0 scope link
default via 10.0.0.1 dev enp1s0
`
	conflicts := explicitConflicts(peer, parseLinuxRoutes(table))
	if len(conflicts) != 1 {
		t.Fatalf("got %v", conflicts)
	}
}

func TestExplicitConflictsOverlappingPrefix(t *testing.T) {
	peer := netip.MustParseAddr("10.77.0.2")
	table := `
10.77.0.0/24 via 10.0.0.1 dev enp1s0
default via 10.0.0.1 dev enp1s0
`
	conflicts := explicitConflicts(peer, parseLinuxRoutes(table))
	if len(conflicts) != 1 {
		t.Fatalf("got %v", conflicts)
	}
}

func TestExplicitConflictsTailscaleStyleOverlap(t *testing.T) {
	peer := netip.MustParseAddr("10.77.0.5")
	table := `
100.64.0.0/10 dev tailscale0 scope link
10.77.0.0/16 dev tailscale0 scope link
`
	conflicts := explicitConflicts(peer, parseLinuxRoutes(table))
	if len(conflicts) != 1 {
		t.Fatalf("got %v", conflicts)
	}
}

func TestParseLinuxRouteLine(t *testing.T) {
	r, ok := parseLinuxRouteLine("default via 10.0.0.1 dev enp1s0")
	if !ok || r.dst.Bits() != 0 {
		t.Fatalf("%+v ok=%v", r, ok)
	}
	r2, ok := parseLinuxRouteLine("10.77.0.1/32 dev mynet0 scope link")
	if !ok || r2.dst.String() != "10.77.0.1/32" {
		t.Fatalf("%+v", r2)
	}
}
