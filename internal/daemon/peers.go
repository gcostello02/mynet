package daemon

import (
	"context"
	"errors"
	"math/rand"
	"net"
	"time"

	"github.com/grantcostello/mynet/internal/client"
	"github.com/grantcostello/mynet/internal/control"
	"net/netip"
)

const (
	peerStatePending    = "Pending"
	peerStateConnecting = "Connecting"
	peerStateWaiting    = "Waiting for peer"
	peerStateTransport  = "Transport ready"
)

func (n *Node) startPeerWorkers(ctx context.Context, rt *client.Runtime, selfID string, peers []control.VPNPeerConfig) {
	for _, p := range peers {
		n.setPeerStatus(p.Name, p.VPNIPv4, p.WGPublicKey, peerStatePending, "", false)
		go n.runPeerWorker(ctx, rt, selfID, p)
	}
}

func (n *Node) runPeerWorker(ctx context.Context, rt *client.Runtime, selfID string, p control.VPNPeerConfig) {
	peerIP := netip.MustParseAddr(p.VPNIPv4)
	backoff := time.Second
	const maxBackoff = 2 * time.Minute
	for {
		if ctx.Err() != nil {
			return
		}
		if !n.VPNActive() {
			return
		}
		n.setPeerStatus(p.Name, p.VPNIPv4, p.WGPublicKey, peerStateConnecting, "", false)
		link, err := n.tryPeerICE(ctx, rt, selfID, p)
		if err == nil && link != nil {
			n.mu.Lock()
			bind := n.bind
			n.mu.Unlock()
			if bind != nil {
				bind.SetPeerConn(peerIP, link.Conn)
			}
			n.setPeerStatus(p.Name, p.VPNIPv4, p.WGPublicKey, peerStateTransport, link.Path.Connection, link.Path.TURNUsed)
			backoff = time.Second
			if !waitPeerConn(ctx, link.Conn) {
				return
			}
			continue
		}
		n.setPeerStatus(p.Name, p.VPNIPv4, p.WGPublicKey, peerStateWaiting, peerErrString(err), false)
		jitter := time.Duration(rand.Int63n(int64(backoff / 2)))
		sleep := backoff + jitter
		if sleep > maxBackoff {
			sleep = maxBackoff
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(sleep):
		}
		if backoff < maxBackoff {
			backoff *= 2
		}
	}
}

// waitPeerConn blocks until the ICE conn closes or ctx is cancelled. Returns false on shutdown.
func waitPeerConn(ctx context.Context, c net.Conn) bool {
	buf := make([]byte, 1)
	for {
		if ctx.Err() != nil {
			return false
		}
		_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, err := c.Read(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return true
		}
	}
}

func (n *Node) tryPeerICE(ctx context.Context, rt *client.Runtime, selfID string, p control.VPNPeerConfig) (*client.PeerLink, error) {
	if l, ok := rt.PeerLink(p.Name); ok {
		return l, nil
	}
	if shouldInitiateICE(selfID, p.DeviceID) {
		attemptCtx, cancel := context.WithTimeout(ctx, 50*time.Second)
		defer cancel()
		return rt.ConnectPeerVPNContext(attemptCtx, p.Name)
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if l, ok := rt.PeerLink(p.Name); ok {
			return l, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil, context.DeadlineExceeded
}

func peerErrString(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "peer offline or unreachable"
	}
	return err.Error()
}

func (n *Node) setPeerStatus(name, vpnIP, wgKey, state, transport string, turn bool) {
	n.mu.Lock()
	if n.peers == nil {
		n.peers = make(map[string]PeerStatus)
	}
	n.peers[name] = PeerStatus{
		Name:        name,
		VPNIP:       vpnIP,
		State:       state,
		Transport:   transport,
		TURNUsed:    turn,
		WGPublicKey: wgKey,
	}
	n.mu.Unlock()
}
