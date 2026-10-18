package wgbind

import (
	"fmt"
	"net"
	"net/netip"
	"testing"

	"golang.zx2c4.com/wireguard/conn"
)

const packetPoolTestCount = 200

func packetPayload(seq int) []byte {
	p := make([]byte, 1400)
	p[0] = byte(seq)
	p[1] = byte(seq >> 8)
	for j := 2; j < len(p); j++ {
		p[j] = byte(seq ^ j)
	}
	return p
}

func verifyPayload(buf []byte, n int, seq int) error {
	exp := packetPayload(seq)
	if n != len(exp) {
		return fmt.Errorf("len %d want %d", n, len(exp))
	}
	for i := 0; i < n; i++ {
		if buf[i] != exp[i] {
			return fmt.Errorf("byte %d: %02x want %02x", i, buf[i], exp[i])
		}
	}
	return nil
}

// TestPacketPoolRecvIntegrity drives the production receive path (Open → recv4) over
// loopback UDP with lockstep backpressure. UDP does not guarantee delivery under blind
// bursts while readLoop blocks on a full inbound channel (see TestPacketPoolRecvIntegrityReliable
// for a lossless net.Pipe stress test).
func TestPacketPoolRecvIntegrity(t *testing.T) {
	ip0 := netip.MustParseAddr("10.88.0.1")
	ip1 := netip.MustParseAddr("10.88.0.2")
	bind0, bind1, cleanup := DuplexPairUDP(ip0, ip1)
	defer cleanup()

	fns, _, err := bind0.Open(0)
	if err != nil {
		t.Fatal(err)
	}
	recv4 := fns[0]
	packets := make([][]byte, 1)
	packets[0] = make([]byte, maxWGPacket)
	sizes := make([]int, 1)
	eps := make([]conn.Endpoint, 1)

	conn := bind1.byIP[ip0].conn
	for seq := 0; seq < packetPoolTestCount; seq++ {
		if _, err := conn.Write(packetPayload(seq)); err != nil {
			t.Fatal(err)
		}
		got, err := recv4(packets, sizes, eps)
		if err != nil {
			t.Fatalf("seq %d recv: %v", seq, err)
		}
		if got != 1 {
			t.Fatalf("seq %d: recv4 returned %d", seq, got)
		}
		if err := verifyPayload(packets[0], sizes[0], seq); err != nil {
			t.Fatalf("seq %d: %v", seq, err)
		}
	}
}

// TestPacketPoolRecvIntegrityReliable uses net.Pipe (no kernel UDP drops) to stress
// buffer pool reuse and detect premature return-to-pool corruption.
func TestPacketPoolRecvIntegrityReliable(t *testing.T) {
	ip1 := netip.MustParseAddr("10.89.0.3")
	a, b := net.Pipe()
	bind0 := NewICEBind()
	bind0.SetPeerConn(ip1, a)
	fns, _, err := bind0.Open(0)
	if err != nil {
		t.Fatal(err)
	}
	recv4 := fns[0]
	packets := make([][]byte, 1)
	packets[0] = make([]byte, maxWGPacket)
	sizes := make([]int, 1)
	eps := make([]conn.Endpoint, 1)

	for seq := 0; seq < packetPoolTestCount; seq++ {
		payload := packetPayload(seq)
		if _, err := b.Write(payload); err != nil {
			t.Fatal(err)
		}
		got, err := recv4(packets, sizes, eps)
		if err != nil || got != 1 {
			t.Fatalf("seq %d: got=%d err=%v", seq, got, err)
		}
		if err := verifyPayload(packets[0], sizes[0], seq); err != nil {
			t.Fatalf("seq %d: %v", seq, err)
		}
	}
	_ = bind0.Shutdown()
	_ = b.Close()
}

func TestPacketPoolPutRejectsWrongCap(t *testing.T) {
	small := make([]byte, 64)
	putPacketBuf(small) // must not poison pool
	buf := getPacketBuf()
	if len(buf) != maxWGPacket {
		t.Fatalf("len %d", len(buf))
	}
	putPacketBuf(buf)
}

func TestPacketPoolAllocsPerRecv(t *testing.T) {
	ip1 := netip.MustParseAddr("10.89.0.2")
	a, b := net.Pipe()
	bind0 := NewICEBind()
	bind0.SetPeerConn(ip1, a)
	_, _, err := bind0.Open(0)
	if err != nil {
		t.Fatal(err)
	}

	payload := make([]byte, 1280)
	allocs := testing.AllocsPerRun(100, func() {
		_, _ = b.Write(payload)
		pkt, ok := <-bind0.inbound
		if !ok {
			t.Fatal("inbound closed")
		}
		_ = copy(make([]byte, pkt.n), pkt.buf[:pkt.n])
		putPacketBuf(pkt.buf)
	})
	// Pool removes the per-packet append([]byte(nil), ...); remaining allocs are channel/pool overhead.
	if allocs > 2 {
		t.Fatalf("expected <=2 allocs per packet through readLoop, got %v", allocs)
	}
	_ = bind0.Shutdown()
	_ = b.Close()
}
