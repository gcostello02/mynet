package client

import (
	"context"
	"errors"
	"net"
	"time"

	icepkg "github.com/gcostello02/mynet/internal/ice"
)

// PeerLink is a nominated ICE transport for VPN WireGuard datagrams.
type PeerLink struct {
	PeerName  string
	Conn      net.Conn
	Path      icepkg.PathInfo
	SessionID string
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

func (r *Runtime) storePeerLink(peerName, sessionID string, conn net.Conn, path icepkg.PathInfo) {
	r.mu.Lock()
	if r.peerLinks == nil {
		r.peerLinks = make(map[string]*PeerLink)
	}
	r.peerLinks[peerName] = &PeerLink{
		PeerName:  peerName,
		Conn:      conn,
		Path:      path,
		SessionID: sessionID,
	}
	ch := r.vpnWait[peerName]
	r.mu.Unlock()
	if ch != nil {
		ch <- nil
	}
}

func (r *Runtime) failPeerVPN(peerName string, err error) {
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
