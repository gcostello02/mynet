package client

import (
	"errors"
	"net"
	"time"

	icepkg "github.com/grantcostello/mynet/internal/ice"
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
	if err := r.load(); err != nil {
		return nil, err
	}
	if r.api.Token == "" {
		return nil, errors.New("not logged in")
	}
	r.SetVPNMode(true)
	if err := r.ensureWebSocket(); err != nil {
		return nil, err
	}
	waitCh := make(chan error, 1)
	r.mu.Lock()
	r.vpnWait = map[string]chan error{peerName: waitCh}
	r.mu.Unlock()

	_, err := r.api.CreateSession(peerName)
	if err != nil {
		return nil, err
	}
	select {
	case err := <-waitCh:
		if err != nil {
			return nil, err
		}
	case <-time.After(45 * time.Second):
		return nil, errors.New("ICE connection timed out")
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
