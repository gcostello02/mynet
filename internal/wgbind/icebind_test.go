package wgbind

import (
	"net/netip"
	"testing"

	"golang.zx2c4.com/wireguard/conn"
)

func TestDuplexPairDatagram(t *testing.T) {
	localIP := netip.MustParseAddr("10.77.0.1")
	remoteIP := netip.MustParseAddr("10.77.0.2")
	l, r, cleanup := DuplexPair(localIP, remoteIP)
	defer cleanup()

	ep := &PeerEndpoint{VPNIP: remoteIP}
	if err := l.Send([][]byte{[]byte("wg-packet")}, ep); err != nil {
		t.Fatal(err)
	}
	fns, _, err := r.Open(0)
	if err != nil {
		t.Fatal(err)
	}
	pkts := [][]byte{make([]byte, 1500)}
	sizes := []int{0}
	eps := make([]conn.Endpoint, 1)
	n, err := fns[0](pkts, sizes, eps)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || string(pkts[0][:sizes[0]]) != "wg-packet" {
		t.Fatalf("got %q", pkts[0][:sizes[0]])
	}
}
