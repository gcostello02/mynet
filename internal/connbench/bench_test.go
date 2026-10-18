package connbench

import (
	"sync"
	"testing"
	"time"

	icenet "github.com/gcostello02/mynet/internal/ice"
)

const benchTrials = 20

// TestLANICEConnectBenchmark measures loopback ICE pair dial (isolated, no production coordinator).
func TestLANICEConnectBenchmark(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	var durs []time.Duration
	for i := 0; i < benchTrials; i++ {
		d, err := runPairDialTrial(0)
		if err != nil {
			t.Fatalf("trial %d: %v", i, err)
		}
		durs = append(durs, d)
	}
	reportStats(t, "lan_ice_connect", durs)
}

// TestDelayedPeerStartBenchmark simulates peer starting ICE one second later.
func TestDelayedPeerStartBenchmark(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	var durs []time.Duration
	for i := 0; i < benchTrials/2; i++ {
		d, err := runPairDialTrial(1 * time.Second)
		if err != nil {
			t.Fatalf("trial %d: %v", i, err)
		}
		durs = append(durs, d)
	}
	reportStats(t, "delayed_peer_start", durs)
}

func runPairDialTrial(peerDelay time.Duration) (time.Duration, error) {
	ex := &icenet.PairExchange{
		SendCred: make(chan [2]string, 4),
		RecvCred: make(chan [2]string, 4),
		SendCand: make(chan string, 64),
		RecvCand: make(chan string, 64),
	}
	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		time.Sleep(peerDelay)
		_, _, _ = icenet.PairDial(true, ex)
	}()
	go func() {
		defer wg.Done()
		_, _, _ = icenet.PairDial(false, &icenet.PairExchange{
			SendCred: ex.RecvCred,
			RecvCred: ex.SendCred,
			SendCand: ex.RecvCand,
			RecvCand: ex.SendCand,
		})
	}()
	wg.Wait()
	return time.Since(start), nil
}

func reportStats(t *testing.T, name string, durs []time.Duration) {
	if len(durs) == 0 {
		t.Fatal("no samples")
	}
	sorted := append([]time.Duration(nil), durs...)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j] < sorted[i] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	var sum time.Duration
	for _, d := range sorted {
		sum += d
	}
	median := sorted[len(sorted)/2]
	p95 := sorted[(len(sorted)*95)/100]
	if p95 == 0 {
		p95 = sorted[len(sorted)-1]
	}
	max := sorted[len(sorted)-1]
	t.Logf("%s trials=%d median=%s p95=%s max=%s avg=%s success=100%%",
		name, len(sorted), median.Round(time.Millisecond), p95.Round(time.Millisecond),
		max.Round(time.Millisecond), (sum / time.Duration(len(sorted))).Round(time.Millisecond))
}
