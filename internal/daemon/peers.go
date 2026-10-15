package daemon

import (
	"context"
	"errors"
	"math/rand"
	"net/netip"
	"time"

	"github.com/gcostello02/mynet/internal/client"
	"github.com/gcostello02/mynet/internal/control"
	icepkg "github.com/gcostello02/mynet/internal/ice"
)

const (
	peerStatePending    = "Pending"
	peerStateConnecting = "Connecting"
	peerStateWaiting    = "Waiting for peer"
	peerStateTransport  = "Transport ready"
)

func (n *Node) startPeerWorkers(ctx context.Context, rt *client.Runtime, selfID string, peers []control.VPNPeerConfig) {
	for _, p := range peers {
		n.setPeerStatus(p.Name, p.VPNIPv4, p.WGPublicKey, peerStatePending, icepkg.PathInfo{}, "")
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
		n.setPeerStatus(p.Name, p.VPNIPv4, p.WGPublicKey, peerStateConnecting, icepkg.PathInfo{}, "")
		link, err := n.tryPeerICE(ctx, rt, selfID, p)
		if err == nil && link != nil {
			n.mu.Lock()
			bind := n.bind
			n.mu.Unlock()
			if bind != nil {
				bind.SetPeerConn(peerIP, link.Conn)
			}
			n.setPeerStatus(p.Name, p.VPNIPv4, p.WGPublicKey, peerStateTransport, link.Path, link.SessionID)
			backoff = time.Second
			if bind == nil {
				select {
				case <-ctx.Done():
					return
				}
			} else if !bind.WaitPeerTransport(ctx, peerIP) {
				return
			}
			rt.ClearPeerLink(p.Name, link.Conn)
			if bind != nil {
				bind.RemovePeer(peerIP)
			}
			n.setPeerStatus(p.Name, p.VPNIPv4, p.WGPublicKey, peerStateConnecting, icepkg.PathInfo{}, "")
			continue
		}
		n.setPeerStatus(p.Name, p.VPNIPv4, p.WGPublicKey, peerStateWaiting, icepkg.PathInfo{}, "")
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

func (n *Node) tryPeerICE(ctx context.Context, rt *client.Runtime, selfID string, p control.VPNPeerConfig) (*client.PeerLink, error) {
	if l, ok := rt.PeerLink(p.Name); ok && l.Conn != nil {
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
		if l, ok := rt.PeerLink(p.Name); ok && l.Conn != nil {
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

func (n *Node) setPeerStatus(name, vpnIP, wgKey, state string, path icepkg.PathInfo, sessionID string) {
	n.mu.Lock()
	if n.peers == nil {
		n.peers = make(map[string]PeerStatus)
	}
	n.peers[name] = PeerStatus{
		Name:          name,
		VPNIP:         vpnIP,
		State:         state,
		Transport:     path.Connection,
		TURNUsed:      path.TURNUsed,
		LocalType:     path.LocalType,
		RemoteType:    path.RemoteType,
		LocalAddr:     path.LocalAddr,
		RemoteAddr:    path.RemoteAddr,
		ICESessionID:  sessionID,
		ICEConnState:  path.ConnectionState,
		WGPublicKey:   wgKey,
	}
	n.mu.Unlock()
}
