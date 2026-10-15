package tundev

import (
	"net/netip"
	"testing"
)

func TestDarwinConflictsDefaultOnly(t *testing.T) {
	peer := netip.MustParseAddr("10.77.0.1")
	netstat := `
Routing tables
Internet:
Destination        Gateway            Flags               Netif Expire
default            10.0.0.1           UGScg                 en0
10.0.0/22          link#14            UCS                   en0
`
	if c := darwinConflicts(peer, netstat); len(c) != 0 {
		t.Fatalf("got %v", c)
	}
}

func TestDarwinConflictsExplicitHostRoute(t *testing.T) {
	peer := netip.MustParseAddr("10.77.0.1")
	netstat := `
Destination        Gateway            Flags               Netif Expire
10.77.0.1          192.168.0.1        UGHS                  en0
default            10.0.0.1           UGScg                 en0
`
	if c := darwinConflicts(peer, netstat); len(c) != 1 {
		t.Fatalf("got %v", c)
	}
}

func TestDarwinIgnoresMACNeighborEntry(t *testing.T) {
	peer := netip.MustParseAddr("10.77.0.2")
	netstat := `
Destination        Gateway            Flags               Netif Expire
10.77.0.2          e0:55:3d:23:18:20  UHLWI                 en0
10/8               link#14            UCS                   en0
`
	if c := darwinConflicts(peer, netstat); len(c) != 1 {
		t.Fatalf("expected connected 10/8 overlap only, got %v", c)
	}
}

func TestDarwinIgnoresTransientHostARP(t *testing.T) {
	peer := netip.MustParseAddr("10.77.0.2")
	netstat := `
Destination        Gateway            Flags               Netif Expire
10.77.0.2          link#14            UHLWIi                en0
10/8               link#14            UCS                   en0
default            10.0.0.1           UGScg                 en0
`
	if c := darwinConflicts(peer, netstat); len(c) != 1 {
		t.Fatalf("expected connected 10/8 overlap only, got %v", c)
	}
}

func TestDarwinConnectedOverlap10slash8(t *testing.T) {
	peer := netip.MustParseAddr("10.77.0.2")
	netstat := `
Destination        Gateway            Flags               Netif Expire
10/8               link#14            UCS                   en0
default            10.0.0.1           UGScg                 en0
`
	if c := darwinConflicts(peer, netstat); len(c) != 1 {
		t.Fatalf("got %v", c)
	}
}

func TestDarwinNoSubstringFalsePositive(t *testing.T) {
	peer := netip.MustParseAddr("10.77.0.1")
	netstat := `
Destination        Gateway            Flags               Netif Expire
10.77.0.10         link#22            UCS                  utun4
default            10.0.0.1           UGScg                 en0
`
	if c := darwinConflicts(peer, netstat); len(c) != 0 {
		t.Fatalf("got %v", c)
	}
}
