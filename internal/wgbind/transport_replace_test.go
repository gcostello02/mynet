package wgbind

import (
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"
)

func TestRapidTransportReplaceSingleReader(t *testing.T) {
	peerIP := netip.MustParseAddr("10.77.1.1")
	bind := NewICEBind()
	defer bind.Shutdown()

	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = bind.Send([][]byte{[]byte("probe")}, &PeerEndpoint{VPNIP: peerIP})
		}
	}()

	for i := 0; i < 50; i++ {
		a, b := net.Pipe()
		bind.SetPeerConn(peerIP, a)
		go func(c net.Conn) {
			buf := make([]byte, 64)
			_, _ = c.Read(buf)
		}(b)
		time.Sleep(time.Millisecond)
		_ = b.Close()
	}
	close(stop)
	wg.Wait()

	st := bind.Stats()
	if len(st.Peers) == 0 {
		t.Fatal("expected peer stats")
	}
}

func TestStaleReadLoopDoesNotDeliverAfterReplace(t *testing.T) {
	peerIP := netip.MustParseAddr("10.77.2.1")
	a1, b1 := net.Pipe()
	a2, b2 := net.Pipe()
	bind := NewICEBind()
	defer bind.Shutdown()

	bind.SetPeerConn(peerIP, a1)
	_, _, _ = bind.Open(0)
	time.Sleep(20 * time.Millisecond)
	bind.SetPeerConn(peerIP, a2)

	done := make(chan struct{})
	go func() {
		_, _ = b1.Write([]byte("stale"))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stale writer blocked")
	}
	time.Sleep(50 * time.Millisecond)
	st := bind.Stats()
	if st.Peers[0].RecvPackets > 0 {
		t.Fatalf("stale read loop delivered packets: %d", st.Peers[0].RecvPackets)
	}
	_ = b2.Close()
}
