package client

import (
	"net"
	"testing"
	"time"

	icepkg "github.com/gcostello02/mynet/internal/ice"
)

type genConn struct{}

func (g *genConn) Read(b []byte) (int, error)  { return 0, nil }
func (g *genConn) Write(b []byte) (int, error) { return len(b), nil }
func (g *genConn) Close() error                { return nil }
func (g *genConn) LocalAddr() net.Addr         { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)} }
func (g *genConn) RemoteAddr() net.Addr        { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 2)} }
func (g *genConn) SetDeadline(time.Time) error       { return nil }
func (g *genConn) SetReadDeadline(time.Time) error   { return nil }
func (g *genConn) SetWriteDeadline(time.Time) error  { return nil }

func TestStorePeerLinkIncrementsGeneration(t *testing.T) {
	rt := NewRuntime(t.TempDir(), nil)
	rt.SetVPNMode(true)
	sess, err := icepkg.NewSession(icepkg.SessionConfig{Controlling: true, LoopbackOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	rt.storePeerLink("peer", "s1", &genConn{}, icepkg.PathInfo{}, sess)
	l1, _ := rt.PeerLink("peer")
	rt.storePeerLink("peer", "s2", &genConn{}, icepkg.PathInfo{}, sess)
	l2, _ := rt.PeerLink("peer")
	if l2.SessionID != "s2" {
		t.Fatal("expected new session id")
	}
	if l2.LinkGen <= l1.LinkGen {
		t.Fatal("expected link generation to increase")
	}
}
