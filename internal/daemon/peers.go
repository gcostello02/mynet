package daemon

import (
	"context"
	"errors"
	"math/rand"
	"net/netip"
	"time"

	"github.com/grantcostello/mynet/internal/client"
	"github.com/grantcostello/mynet/internal/control"
	icepkg "github.com/grantcostello/mynet/internal/ice"
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
		peerCtx, cancel := context.WithCancel(ctx)
		n.mu.Lock()
		if n.peerWorkers == nil {
			n.peerWorkers = make(map[string]context.CancelFunc)
		}
		if old, ok := n.peerWorkers[p.Name]; ok {
			old()
		}
		n.peerWorkers[p.Name] = cancel
		n.mu.Unlock()
		go n.runPeerWorker(peerCtx, rt, selfID, p)
	}
}

func (n *Node) isAuthorizedVPNPeer(name string) bool {
	n.mu.Lock()
	_, ok := n.vpnPeerKeys[name]
	n.mu.Unlock()
	return ok
}

func (n *Node) stopVPNPeerWorker(peerName string) {
	n.mu.Lock()
	cancel := n.peerWorkers[peerName]
	delete(n.peerWorkers, peerName)
	n.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (n *Node) runPeerWorker(ctx context.Context, rt *client.Runtime, selfID string, p control.VPNPeerConfig) {
	peerIP := netip.MustParseAddr(p.VPNIPv4)
	backoff := time.Second
	const maxBackoff = 2 * time.Minute
	iceFailures := 0
	transportRecoveries := 0
	rt.BeginPeerRecoveryCycle(p.Name)
	for {
		if ctx.Err() != nil {
			return
		}
		if !n.VPNActive() || !n.isAuthorizedVPNPeer(p.Name) {
			return
		}
		n.setPeerRecovery(p.Name, iceFailures, transportRecoveries, "connecting")
		n.setPeerStatus(p.Name, p.VPNIPv4, p.WGPublicKey, peerStateConnecting, icepkg.PathInfo{}, "")
		link, iceErr := n.tryPeerICE(ctx, rt, selfID, p)
		if iceErr == nil && link != nil {
			n.mu.Lock()
			bind := n.bind
			eng := n.engine
			running := n.running
			n.mu.Unlock()
			if bind != nil {
				bind.SetPeerConn(peerIP, link.Conn)
			}
			if running && eng != nil {
				if err := eng.NudgeHandshake(p.WGPublicKey); err != nil && n.Logger != nil {
					n.Logger.Debug("wireguard handshake nudge", "peer", p.Name, "err", err)
				}
			}
			livePath, _ := rt.PeerLinkLivePath(p.Name)
			if livePath.ConnectionState == "" {
				livePath = link.Path
			}
			n.setPeerRecovery(p.Name, iceFailures, transportRecoveries, "stable")
			n.setPeerStatus(p.Name, p.VPNIPv4, p.WGPublicKey, peerStateTransport, livePath, link.SessionID)
			wgCtx, wgCancel := context.WithTimeout(ctx, 30*time.Second)
			go func() {
				observeWireGuardReadiness(wgCtx, rt, eng, p.Name, link.SessionID, p.WGPublicKey)
				wgCancel()
			}()
			backoff = time.Second
			if bind == nil {
				select {
				case <-ctx.Done():
					return
				}
			} else if !bind.WaitPeerTransport(ctx, peerIP) {
				return
			}
			transportRecoveries++
			rt.NoteTransportRecovery(p.Name)
			rt.BeginPeerRecoveryCycle(p.Name)
			n.setPeerRecovery(p.Name, iceFailures, transportRecoveries, "transport-down")
			rt.ClearPeerLink(p.Name, link.Conn)
			if bind != nil {
				bind.RemovePeer(peerIP)
			}
			n.setPeerStatus(p.Name, p.VPNIPv4, p.WGPublicKey, peerStateConnecting, icepkg.PathInfo{}, "")
			continue
		}
		if peerICEErrorCountsAsFailure(iceErr) {
			rt.NoteICEAttemptFailed(p.Name, peerErrString(iceErr))
		}
		if tl, ok := rt.PeerRecoveryTimeline(p.Name); ok {
			iceFailures = tl.ICEAttemptsFailed
		}
		n.setPeerRecovery(p.Name, iceFailures, transportRecoveries, "waiting")
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
	if rt.PeerLinkStale(p.Name) {
		if l, ok := rt.PeerLink(p.Name); ok {
			rt.ClearPeerLink(p.Name, l.Conn)
		}
	} else if l, ok := rt.PeerLink(p.Name); ok && l.Conn != nil {
		return l, nil
	}
	if shouldInitiateICE(selfID, p.DeviceID) {
		attemptCtx, cancel := context.WithTimeout(ctx, 50*time.Second)
		defer cancel()
		return rt.ConnectPeerVPNContext(attemptCtx, p.Name)
	}
	deadline := time.Now().Add(50 * time.Second)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if rt.PeerLinkStale(p.Name) {
			if l, ok := rt.PeerLink(p.Name); ok {
				rt.ClearPeerLink(p.Name, l.Conn)
			}
		} else if l, ok := rt.PeerLink(p.Name); ok && l.Conn != nil {
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

func (n *Node) setPeerRecovery(name string, iceFailures, transportRecoveries int, state string) {
	n.mu.Lock()
	if n.peers == nil {
		n.peers = make(map[string]PeerStatus)
	}
	if p, ok := n.peers[name]; ok {
		p.ICEFailedAttempts = iceFailures
		p.TransportRecoveries = transportRecoveries
		p.RecoveryAttempts = iceFailures
		p.RecoveryState = state
		n.peers[name] = p
	} else {
		n.peers[name] = PeerStatus{
			Name:                name,
			ICEFailedAttempts:   iceFailures,
			TransportRecoveries: transportRecoveries,
			RecoveryAttempts:    iceFailures,
			RecoveryState:       state,
		}
	}
	n.mu.Unlock()
}

func (n *Node) setPeerStatus(name, vpnIP, wgKey, state string, path icepkg.PathInfo, sessionID string) {
	n.mu.Lock()
	if n.peers == nil {
		n.peers = make(map[string]PeerStatus)
	}
	iceFailures := 0
	transportRecoveries := 0
	recoveryState := ""
	if prev, ok := n.peers[name]; ok {
		iceFailures = prev.ICEFailedAttempts
		transportRecoveries = prev.TransportRecoveries
		recoveryState = prev.RecoveryState
	}
	n.peers[name] = PeerStatus{
		Name:                name,
		VPNIP:               vpnIP,
		State:               state,
		Transport:           path.Connection,
		TURNUsed:            path.TURNUsed,
		LocalType:           path.LocalType,
		RemoteType:          path.RemoteType,
		LocalAddr:           path.LocalAddr,
		RemoteAddr:          path.RemoteAddr,
		ICESessionID:        sessionID,
		ICEConnState:        path.ConnectionState,
		ICEFailedAttempts:   iceFailures,
		TransportRecoveries: transportRecoveries,
		RecoveryAttempts:    iceFailures,
		RecoveryState:       recoveryState,
		WGPublicKey:         wgKey,
	}
	n.mu.Unlock()
}
