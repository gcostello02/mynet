package wgbind

import (
	"bytes"
	"net"
	"net/netip"
	"testing"
	"time"
)

func TestPacketPoolRecvIntegrity(t *testing.T) {
	ip0 := netip.MustParseAddr("10.88.0.1")
	ip1 := netip.MustParseAddr("10.88.0.2")
	bind0, bind1, cleanup := DuplexPairUDP(ip0, ip1)
	defer cleanup()

	payload := bytes.Repeat([]byte{0xab}, 1400)
	conn := bind1.byIP[ip0].conn
	for i := 0; i < 200; i++ {
		if _, err := conn.Write(payload); err != nil {
			t.Fatal(err)
		}
	}

	bind0.mu.RLock()
	inbound := bind0.inbound
	bind0.mu.RUnlock()
	if inbound == nil {
		t.Fatal("inbound nil")
	}

	deadline := time.After(2 * time.Second)
	for i := 0; i < 200; i++ {
		select {
		case pkt := <-inbound:
			if !bytes.Equal(pkt.buf[:pkt.n], payload) {
				t.Fatalf("packet %d mismatch", i)
			}
			putPacketBuf(pkt.buf)
		case <-deadline:
			t.Fatalf("only received %d packets", i)
		}
	}
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
