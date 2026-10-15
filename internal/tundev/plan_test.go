package tundev

import (
	"net/netip"
	"testing"
)

func TestBuildPlanPeerRoutes(t *testing.T) {
	local := netip.MustParsePrefix("10.77.0.2/32")
	peers := []netip.Addr{
		netip.MustParseAddr("10.77.0.1"),
		netip.MustParseAddr("10.77.0.3"),
	}
	plan := BuildPlan(local, peers)
	if len(plan.PeerRoutes) != 2 {
		t.Fatalf("routes: %v", plan.PeerRoutes)
	}
	if plan.LocalPrefix != "10.77.0.2/32" {
		t.Fatalf("local: %s", plan.LocalPrefix)
	}
}
