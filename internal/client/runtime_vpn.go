package client

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"time"

	icepkg "github.com/grantcostello/mynet/internal/ice"
)

// PeerLink is a nominated ICE transport for VPN WireGuard datagrams.
type PeerLink struct {
	PeerName  string
	Conn      net.Conn
	Path      icepkg.PathInfo
	SessionID string
	Session   *icepkg.Session
	LinkGen   uint64
}

// SetVPNMode enables multi-peer ICE links without MN application framing on conns.
func (r *Runtime) SetVPNMode(enabled bool) {
	r.mu.Lock()
	r.vpnMode = enabled
	if r.peerLinks == nil {
		r.peerLinks = make(map[string]*PeerLink)
	}
	r.mu.Unlock()
}

func (r *Runtime) PeerLink(peerName string) (*PeerLink, bool) {
	r.mu.Lock()
	l, ok := r.peerLinks[peerName]
	r.mu.Unlock()
	return l, ok
}

// ConnectPeerVPN establishes ICE to a peer for WireGuard transport (initiator).
func (r *Runtime) ConnectPeerVPN(peerName string) (*PeerLink, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	return r.ConnectPeerVPNContext(ctx, peerName)
}

// ConnectPeerVPNContext is like ConnectPeerVPN but respects cancellation and avoids duplicate sessions.
func (r *Runtime) ConnectPeerVPNContext(ctx context.Context, peerName string) (*PeerLink, error) {
	if err := r.load(); err != nil {
		return nil, err
	}
	if r.api.Token == "" {
		return nil, errors.New("not logged in")
	}
	if l, ok := r.PeerLink(peerName); ok {
		return l, nil
	}
	r.SetVPNMode(true)
	if err := r.waitForSignaling(ctx); err != nil {
		return nil, err
	}
	r.waitPeerOnlineForICE(ctx, peerName)
	r.mu.Lock()
	if r.vpnConnecting == nil {
		r.vpnConnecting = make(map[string]bool)
	}
	if r.vpnConnecting[peerName] {
		waitCh := r.vpnWait[peerName]
		r.mu.Unlock()
		if waitCh == nil {
			return nil, errors.New("peer connection already in progress")
		}
		select {
		case err := <-waitCh:
			if err != nil {
				return nil, err
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		link, ok := r.PeerLink(peerName)
		if !ok {
			return nil, errors.New("peer link missing after ICE")
		}
		return link, nil
	}
	waitCh := make(chan error, 1)
	if r.vpnWait == nil {
		r.vpnWait = make(map[string]chan error)
	}
	r.vpnWait[peerName] = waitCh
	r.vpnConnecting[peerName] = true
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		delete(r.vpnConnecting, peerName)
		delete(r.vpnWait, peerName)
		r.mu.Unlock()
	}()

	r.cancelPeerICEAttempt(peerName)
	_, err := r.api.CreateSession(peerName)
	if err != nil {
		return nil, err
	}
	select {
	case err := <-waitCh:
		if err != nil {
			return nil, err
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	link, ok := r.PeerLink(peerName)
	if !ok {
		return nil, errors.New("peer link missing after ICE")
	}
	return link, nil
}

func (r *Runtime) nextPeerLinkGen() uint64 {
	return atomic.AddUint64(&r.peerLinkGen, 1)
}

// peerOnlineICEMaxWait is the upper bound for waitPeerOnlineForICE (overridable in tests).
var peerOnlineICEMaxWait = 12 * time.Second

// waitPeerOnlineForICE reduces credential-timeout races when the remote daemon is still starting.
func (r *Runtime) waitPeerOnlineForICE(ctx context.Context, peerName string) {
	deadline := time.Now().Add(peerOnlineICEMaxWait)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return
		}
		peers, err := r.api.Peers()
		if err == nil {
			for _, p := range peers {
				if p.Name == peerName && p.Online {
					return
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (r *Runtime) storePeerLink(peerName, sessionID string, conn net.Conn, path icepkg.PathInfo, sess *icepkg.Session) {
	gen := r.nextPeerLinkGen()
	r.mu.Lock()
	if r.peerLinks == nil {
		r.peerLinks = make(map[string]*PeerLink)
	}
	if prev, ok := r.peerLinks[peerName]; ok {
		if prev.Conn != nil && prev.Conn != conn {
			_ = prev.Conn.Close()
		}
		if prev.Session != nil && prev.Session != sess {
			_ = prev.Session.Close()
		}
	}
	r.peerLinks[peerName] = &PeerLink{
		PeerName:  peerName,
		Conn:      conn,
		Path:      path,
		SessionID: sessionID,
		Session:   sess,
		LinkGen:   gen,
	}
	ch := r.vpnWait[peerName]
	r.mu.Unlock()

	if sess != nil {
		sess.SetStateChangeHandler(func(st icepkg.ConnectionState) {
			if !icepkg.StateTerminal(st) {
				return
			}
			r.mu.Lock()
			cur, ok := r.peerLinks[peerName]
			if !ok || cur.SessionID != sessionID || cur.LinkGen != gen {
				r.mu.Unlock()
				return
			}
			c := cur.Conn
			r.mu.Unlock()
			if c != nil {
				_ = c.Close()
			}
		})
	}

	if ch != nil {
		ch <- nil
	}
}

// PeerLinkLivePath returns up-to-date ICE path info for a VPN peer when available.
func (r *Runtime) PeerLinkLivePath(peerName string) (icepkg.PathInfo, bool) {
	r.mu.Lock()
	l, ok := r.peerLinks[peerName]
	r.mu.Unlock()
	if !ok || l == nil {
		return icepkg.PathInfo{}, false
	}
	if l.Session != nil {
		return l.Session.LivePath(), true
	}
	return l.Path, true
}

// PeerLinkStale reports whether the cached link should not be reused.
func (r *Runtime) PeerLinkStale(peerName string) bool {
	r.mu.Lock()
	l, ok := r.peerLinks[peerName]
	r.mu.Unlock()
	if !ok || l == nil || l.Conn == nil {
		return true
	}
	if l.Session != nil && icepkg.StateTerminal(l.Session.State()) {
		return true
	}
	return false
}

func (r *Runtime) failPeerVPN(peerName string, err error) {
	if err != nil {
		r.NoteICEAttemptFailed(peerName, err.Error())
	}
	r.mu.Lock()
	ch := r.vpnWait[peerName]
	r.mu.Unlock()
	if ch != nil {
		ch <- err
	}
}

// ClearPeerLink removes a cached VPN ICE link after its transport closes.
func (r *Runtime) ClearPeerLink(peerName string, conn net.Conn) {
	r.mu.Lock()
	if l, ok := r.peerLinks[peerName]; ok && (conn == nil || l.Conn == conn) {
		delete(r.peerLinks, peerName)
	}
	r.mu.Unlock()
}

// InvalidateAllPeerTransports closes active VPN ICE links after a host network change.
func (r *Runtime) InvalidateAllPeerTransports() {
	r.mu.Lock()
	links := r.peerLinks
	r.peerLinks = make(map[string]*PeerLink)
	r.mu.Unlock()
	for _, l := range links {
		if l != nil && l.Conn != nil {
			_ = l.Conn.Close()
		}
	}
}
