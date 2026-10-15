package wgbind

import (
	"net"
	"sync"
	"time"
)

// udpPair returns two connected UDP sessions suitable for ICE-like duplex tests.
func udpPair() (net.Conn, net.Conn, func()) {
	a, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		panic(err)
	}
	b, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		_ = a.Close()
		panic(err)
	}
	ca := &udpSession{conn: a, peer: b.LocalAddr().(*net.UDPAddr)}
	cb := &udpSession{conn: b, peer: a.LocalAddr().(*net.UDPAddr)}
	cleanup := func() {
		_ = a.Close()
		_ = b.Close()
	}
	return ca, cb, cleanup
}

type udpSession struct {
	conn *net.UDPConn
	peer *net.UDPAddr
	mu   sync.Mutex
}

func (u *udpSession) Read(b []byte) (int, error) {
	for {
		n, addr, err := u.conn.ReadFromUDP(b)
		if err != nil {
			return n, err
		}
		if addr.String() == u.peer.String() {
			return n, nil
		}
	}
}

func (u *udpSession) Write(b []byte) (int, error) {
	return u.conn.WriteToUDP(b, u.peer)
}

func (u *udpSession) Close() error {
	return u.conn.Close()
}

func (u *udpSession) LocalAddr() net.Addr  { return u.conn.LocalAddr() }
func (u *udpSession) RemoteAddr() net.Addr { return u.peer }
func (u *udpSession) SetDeadline(t time.Time) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.conn.SetDeadline(t)
}
func (u *udpSession) SetReadDeadline(t time.Time) error  { return u.conn.SetReadDeadline(t) }
func (u *udpSession) SetWriteDeadline(t time.Time) error { return u.conn.SetWriteDeadline(t) }
