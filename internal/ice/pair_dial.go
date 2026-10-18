package icenet

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

// PairExchange is in-memory signaling for two local ICE sessions (tests).
type PairExchange struct {
	SendCred chan [2]string
	RecvCred chan [2]string
	SendCand chan string // empty string means local gathering complete
	RecvCand chan string
}

// PairDial runs one side of a loopback ICE negotiation with gather synchronization.
func PairDial(controlling bool, ex *PairExchange) (*Session, net.Conn, error) {
	if ex == nil {
		return nil, nil, errors.New("nil PairExchange")
	}
	cfg := SessionConfig{Controlling: controlling}
	sess, err := NewSession(cfg)
	if err != nil {
		return nil, nil, err
	}

	var localGatherDone bool
	var gatherMu sync.Mutex
	remoteGatherDone := make(chan struct{}, 1)

	stopCand := make(chan struct{})
	var candWG sync.WaitGroup
	candWG.Add(1)
	go func() {
		defer candWG.Done()
		for {
			select {
			case <-stopCand:
				return
			case cand, ok := <-ex.RecvCand:
				if !ok {
					return
				}
				if cand == "" {
					select {
					case remoteGatherDone <- struct{}{}:
					default:
					}
					continue
				}
				_ = sess.AddRemoteCandidate(cand)
			}
		}
	}()

	if err := sess.SetCandidateHandler(func(c string) {
		if c == "" {
			gatherMu.Lock()
			localGatherDone = true
			gatherMu.Unlock()
			ex.SendCand <- ""
			return
		}
		ex.SendCand <- c
	}); err != nil {
		close(stopCand)
		candWG.Wait()
		_ = sess.Close()
		return nil, nil, err
	}

	ufrag, pwd, err := sess.LocalCredentials()
	if err != nil {
		close(stopCand)
		candWG.Wait()
		_ = sess.Close()
		return nil, nil, err
	}
	ex.SendCred <- [2]string{ufrag, pwd}
	remote := <-ex.RecvCred

	if err := sess.Gather(); err != nil {
		close(stopCand)
		candWG.Wait()
		_ = sess.Close()
		return nil, nil, err
	}
	if err := waitLocalGather(&gatherMu, &localGatherDone, 8*time.Second); err != nil {
		close(stopCand)
		candWG.Wait()
		_ = sess.Close()
		return nil, nil, err
	}
	select {
	case <-remoteGatherDone:
	case <-time.After(8 * time.Second):
		close(stopCand)
		candWG.Wait()
		_ = sess.Close()
		return nil, nil, errors.New("remote ICE gathering did not complete")
	}
	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	conn, err := sess.Connect(ctx, remote[0], remote[1])
	close(stopCand)
	candWG.Wait()
	if err != nil {
		_ = sess.Close()
		return nil, nil, err
	}
	return sess, conn, nil
}

func waitLocalGather(mu *sync.Mutex, done *bool, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		mu.Lock()
		ok := *done
		mu.Unlock()
		if ok {
			return nil
		}
		time.Sleep(15 * time.Millisecond)
	}
	return errors.New("local ICE gathering did not complete")
}
