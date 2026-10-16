package icenet

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/grantcostello/mynet/internal/peer"
)

func TestLocalICEPairPing(t *testing.T) {
	ex := &PairExchange{
		SendCred: make(chan [2]string, 1),
		RecvCred: make(chan [2]string, 1),
		SendCand: make(chan string, 64),
		RecvCand: make(chan string, 64),
	}
	// cross-wire B's channels
	exB := &PairExchange{
		SendCred: ex.RecvCred,
		RecvCred: ex.SendCred,
		SendCand: ex.RecvCand,
		RecvCand: ex.SendCand,
	}

	var wg sync.WaitGroup
	wg.Add(2)
	var connA, connB net.Conn
	var sessA, sessB *Session
	var errA, errB error

	go func() {
		defer wg.Done()
		sessA, connA, errA = PairDial(true, ex)
	}()
	go func() {
		defer wg.Done()
		sessB, connB, errB = PairDial(false, exB)
	}()
	wg.Wait()

	if errA != nil {
		t.Fatal("controlling:", errA)
	}
	if errB != nil {
		t.Fatal("controlled:", errB)
	}
	defer func() {
		if connA != nil {
			_ = connA.Close()
		}
		if connB != nil {
			_ = connB.Close()
		}
		if sessA != nil {
			_ = sessA.Close()
		}
		if sessB != nil {
			_ = sessB.Close()
		}
	}()

	stop := make(chan struct{})
	var responderReady sync.WaitGroup
	responderReady.Add(1)
	go func() {
		responderReady.Done()
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
	responderReady.Wait()

	rtt, err := peer.PingRTT(connA, 5*time.Second)
	close(stop)
	if err != nil {
		t.Fatal(err)
	}
	if rtt <= 0 {
		t.Fatal("expected rtt")
	}
}
