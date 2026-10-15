package icenet

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/grantcostello/mynet/internal/peer"
)

func TestLocalICEPairPing(t *testing.T) {
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
		connA, errA = dialLocalPeer(true, credAtoB, credBtoA, candAtoB, candBtoA)
	}()
	go func() {
		defer wg.Done()
		connB, errB = dialLocalPeer(false, credBtoA, credAtoB, candBtoA, candAtoB)
	}()
	wg.Wait()

	if errA != nil {
		t.Fatal("controlling:", errA)
	}
	if errB != nil {
		t.Fatal("controlled:", errB)
	}

	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			f, err := peer.ReadFrame(connB, time.Second)
			if err != nil {
				continue
			}
			if resp := peer.RespondPing(f); resp != nil {
				_, _ = connB.Write(resp)
			}
		}
	}()

	rtt, err := peer.PingRTT(connA, 5*time.Second)
	close(stop)
	if err != nil {
		t.Fatal(err)
	}
	if rtt <= 0 {
		t.Fatal("expected rtt")
	}
}

func dialLocalPeer(
	controlling bool,
	sendCred, recvCred chan [2]string,
	sendCand, recvCand chan string,
) (net.Conn, error) {
	sess, err := NewSession(SessionConfig{Controlling: controlling})
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
	return sess.Connect(ctx, remote[0], remote[1])
}
