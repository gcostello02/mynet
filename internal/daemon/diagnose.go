package daemon

import (
	"github.com/grantcostello/mynet/internal/client"
	"github.com/grantcostello/mynet/internal/control"
)

// Diagnose returns daemon-owned connectivity state (no extra WebSocket).
func (n *Node) Diagnose(peerName string) (client.Diagnostics, error) {
	n.mu.Lock()
	rt := n.rt
	running := n.running
	peerMap := make(map[string]PeerStatus, len(n.peers))
	for k, v := range n.peers {
		peerMap[k] = v
	}
	n.mu.Unlock()

	if rt == nil {
		fallback := client.NewRuntime(n.StateDir, n.Logger)
		if err := fallback.LoadSessionOnly(); err != nil {
			return client.Diagnostics{}, err
		}
		return fallback.Diagnose(peerName)
	}
	d, err := rt.Diagnose(peerName)
	if err != nil {
		return d, err
	}
	if running {
		d.WireGuardState = "Active"
	}
	if peerName != "" {
		if p, ok := peerMap[peerName]; ok {
			d.ICEState = p.State
			if p.Transport != "" {
				d.Connection = p.Transport
			}
		}
	}
	return d, nil
}

// PeersList uses HTTP only (no WebSocket side effects).
func (n *Node) PeersList() ([]control.PeerInfo, error) {
	n.mu.Lock()
	rt := n.rt
	n.mu.Unlock()
	if rt == nil {
		return nil, nil
	}
	return rt.Peers()
}
