package daemon

import (
	"context"
	"sync"
	"testing"
)

func TestStopVPNPeerWorkerCancelsContext(t *testing.T) {
	n := &Node{peerWorkers: make(map[string]context.CancelFunc)}
	ctx, cancel := context.WithCancel(context.Background())
	n.peerWorkers["client-c"] = cancel
	n.stopVPNPeerWorker("client-c")
	if ctx.Err() == nil {
		t.Fatal("expected peer worker context cancelled")
	}
}

func TestIsAuthorizedVPNPeer(t *testing.T) {
	n := &Node{vpnPeerKeys: map[string]string{"client-a": "abc"}}
	if !n.isAuthorizedVPNPeer("client-a") {
		t.Fatal("expected authorized")
	}
	if n.isAuthorizedVPNPeer("client-c") {
		t.Fatal("expected not authorized")
	}
}

func TestReconcileRemovalPathCancelsWorker(t *testing.T) {
	n := &Node{
		vpnPeerKeys: map[string]string{"gone": "deadbeef"},
		peers: map[string]PeerStatus{
			"gone": {Name: "gone", VPNIP: "10.88.0.9"},
		},
		peerWorkers: make(map[string]context.CancelFunc),
		reconcileMu: sync.Mutex{},
	}
	ctx, cancel := context.WithCancel(context.Background())
	n.peerWorkers["gone"] = cancel
	n.stopVPNPeerWorker("gone")
	delete(n.vpnPeerKeys, "gone")
	delete(n.peers, "gone")
	if ctx.Err() == nil {
		t.Fatal("worker should be cancelled on removal path")
	}
}
