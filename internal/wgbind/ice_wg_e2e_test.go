package wgbind

import (
	"context"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	icepkg "github.com/grantcostello/mynet/internal/ice"
)

func TestWireGuardHandshakeOverRealICE(t *testing.T) {
	ip0 := netip.MustParseAddr("10.88.0.1")
	ip1 := netip.MustParseAddr("10.88.0.2")
	iceA, iceB, cleanup := pairICEConns(t)
	defer cleanup()

	bind0 := NewICEBind()
	bind1 := NewICEBind()
	bind0.SetPeerConn(ip1, iceA)
	bind1.SetPeerConn(ip0, iceB)

	if !encryptedPingOverBinds(t, bind0, bind1, ip0, ip1, 15*time.Second) {
		st0 := bind0.Stats()
		st1 := bind1.Stats()
		t.Fatalf("WG over ICE failed; bind0 peers=%d send=%d recv=%d bind1 send=%d recv=%d",
			len(st0.Peers), sumSend(st0), sumRecv(st0), sumSend(st1), sumRecv(st1))
	}
}

func TestWireGuardICETransportReplacement(t *testing.T) {
	ip0 := netip.MustParseAddr("10.88.0.3")
	ip1 := netip.MustParseAddr("10.88.0.4")
	iceA1, iceB1, cleanup1 := pairICEConns(t)
	defer cleanup1()

	bind0 := NewICEBind()
	bind1 := NewICEBind()
	bind0.SetPeerConn(ip1, iceA1)
	bind1.SetPeerConn(ip0, iceB1)
	if !encryptedPingOverBinds(t, bind0, bind1, ip0, ip1, 15*time.Second) {
		t.Fatal("initial WG over ICE")
	}

	_ = iceA1.Close()
	_ = iceB1.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = bind0.WaitPeerTransport(ctx, ip1)
	_ = bind1.WaitPeerTransport(ctx, ip0)

	iceA2, iceB2, cleanup2 := pairICEConns(t)
	defer cleanup2()
	bind0.SetPeerConn(ip1, iceA2)
	bind1.SetPeerConn(ip0, iceB2)
	if !encryptedPingOverBinds(t, bind0, bind1, ip0, ip1, 15*time.Second) {
		t.Fatal("WG over ICE after transport replacement")
	}
}

func sumSend(st BindStats) uint64 {
	var n uint64
	for _, p := range st.Peers {
		n += p.SendPackets
	}
	return n
}

func sumRecv(st BindStats) uint64 {
	var n uint64
	for _, p := range st.Peers {
		n += p.RecvPackets
	}
	return n
}

func pairICEConns(t *testing.T) (net.Conn, net.Conn, func()) {
	credAtoB := make(chan [2]string, 1)
	credBtoA := make(chan [2]string, 1)
	candAtoB := make(chan string, 32)
	candBtoA := make(chan string, 32)

	var wg sync.WaitGroup
	wg.Add(2)
	var connA, connB net.Conn
	var errA, errB error

	go func() {
		defer wg.Done()
		connA, errA = dialICEPeer(true, credAtoB, credBtoA, candAtoB, candBtoA)
	}()
	go func() {
		defer wg.Done()
		connB, errB = dialICEPeer(false, credBtoA, credAtoB, candBtoA, candAtoB)
	}()
	wg.Wait()

	if errA != nil {
		t.Fatalf("ice A: %v", errA)
	}
	if errB != nil {
		t.Fatalf("ice B: %v", errB)
	}
	return connA, connB, func() {
		_ = connA.Close()
		_ = connB.Close()
	}
}

func dialICEPeer(
	controlling bool,
	sendCred, recvCred chan [2]string,
	sendCand, recvCand chan string,
) (net.Conn, error) {
	sess, err := icepkg.NewSession(icepkg.SessionConfig{Controlling: controlling})
	if err != nil {
		return nil, err
	}
	if err := sess.SetCandidateHandler(func(c string) {
		if c != "" {
			sendCand <- c
		}
	}); err != nil {
		return nil, err
	}
	ufrag, pwd, err := sess.LocalCredentials()
	if err != nil {
		return nil, err
	}
	sendCred <- [2]string{ufrag, pwd}
	remote := <-recvCred

	go func() {
		for cand := range recvCand {
			_ = sess.AddRemoteCandidate(cand)
		}
	}()

	if err := sess.Gather(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ic, err := sess.Connect(ctx, remote[0], remote[1])
	if err != nil {
		return nil, err
	}
	return ic, nil
}
