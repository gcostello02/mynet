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

func TestDarwinConflictsHostRoute(t *testing.T) {
	peer := netip.MustParseAddr("10.77.0.1")
	netstat := `
Destination        Gateway            Flags               Netif Expire
10.77.0.1          link#22            UCS                  utun4
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
