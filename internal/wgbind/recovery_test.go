package wgbind

import (
	"context"
	"io"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/pion/ice/v4"
)

type stallConn struct {
	mu        sync.Mutex
	readBlock chan struct{}
	writeErr  error
	closed    bool
	closeOnce sync.Once
}

func (c *stallConn) Read(b []byte) (int, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return 0, net.ErrClosed
	}
	c.mu.Unlock()
	select {
	case <-c.readBlock:
		return 0, net.ErrClosed
	default:
	}
	time.Sleep(50 * time.Millisecond)
	return 0, &net.OpError{Op: "read", Err: timeoutError{}}
}

func (c *stallConn) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return 0, net.ErrClosed
	}
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	return len(b), nil
}

func (c *stallConn) Close() error {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.closed = true
		c.mu.Unlock()
		close(c.readBlock)
	})
	return nil
}

func (c *stallConn) LocalAddr() net.Addr                { return &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1} }
func (c *stallConn) RemoteAddr() net.Addr               { return &net.UDPAddr{IP: net.IPv4(127, 0, 0, 2), Port: 2} }
func (c *stallConn) SetDeadline(t time.Time) error      { return nil }
func (c *stallConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *stallConn) SetWriteDeadline(t time.Time) error { return nil }

type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func TestWriteFailureWhileReadBlockedClosesTransport(t *testing.T) {
	ip := netip.MustParseAddr("10.77.0.5")
	peerIP := netip.MustParseAddr("10.77.0.6")
	sc := &stallConn{readBlock: make(chan struct{}), writeErr: ice.ErrNoCandidatePairs}
	bind := NewICEBind()
	bind.SetPeerConn(peerIP, sc)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = bind.Send([][]byte{[]byte("wg")}, &PeerEndpoint{VPNIP: peerIP})

	if !bind.WaitPeerTransport(ctx, peerIP) {
		t.Fatal("transport did not close after terminal write failure")
	}
	st := bind.Stats()
	if len(st.Peers) == 0 || st.Peers[0].SendErrors == 0 {
		t.Fatalf("expected send errors, stats=%+v", st.Peers)
	}
	bind.RemovePeer(peerIP)
	_ = ip
}

func TestTransientWriteErrorDoesNotCloseTransport(t *testing.T) {
	peerIP := netip.MustParseAddr("10.77.0.7")
	sc := &stallConn{readBlock: make(chan struct{}), writeErr: timeoutError{}}
	bind := NewICEBind()
	bind.SetPeerConn(peerIP, sc)

	for i := 0; i < consecutiveSendFailureThreshold-1; i++ {
		_ = bind.Send([][]byte{[]byte("x")}, &PeerEndpoint{VPNIP: peerIP})
	}
	st := bind.Stats()
	if !st.Peers[0].HasActiveConn {
		t.Fatal("conn closed after transient errors")
	}
	if st.Peers[0].ConsecutiveErrors != consecutiveSendFailureThreshold-1 {
		t.Fatalf("unexpected consecutive=%d", st.Peers[0].ConsecutiveErrors)
	}
	sc.writeErr = nil
	_ = bind.Send([][]byte{[]byte("ok")}, &PeerEndpoint{VPNIP: peerIP})
	st = bind.Stats()
	if st.Peers[0].ConsecutiveErrors != 0 {
		t.Fatalf("successful send should reset streak, got %d", st.Peers[0].ConsecutiveErrors)
	}
	bind.RemovePeer(peerIP)
}

func TestTerminalICEWriteErrorClosesImmediately(t *testing.T) {
	peerIP := netip.MustParseAddr("10.77.0.8")
	sc := &stallConn{readBlock: make(chan struct{}), writeErr: ice.ErrNoCandidatePairs}
	bind := NewICEBind()
	bind.SetPeerConn(peerIP, sc)
	_ = bind.Send([][]byte{[]byte("x")}, &PeerEndpoint{VPNIP: peerIP})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if !bind.WaitPeerTransport(ctx, peerIP) {
		t.Fatal("expected immediate close on terminal ICE error")
	}
}

func TestRepeatedErrorsTriggerRecovery(t *testing.T) {
	peerIP := netip.MustParseAddr("10.77.0.9")
	sc := &stallConn{readBlock: make(chan struct{}), writeErr: io.ErrClosedPipe}
	bind := NewICEBind()
	bind.SetPeerConn(peerIP, sc)
	_ = bind.Send([][]byte{[]byte("a")}, &PeerEndpoint{VPNIP: peerIP})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if !bind.WaitPeerTransport(ctx, peerIP) {
		t.Fatal("expected transport down")
	}
}
