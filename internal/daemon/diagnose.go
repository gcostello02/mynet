package daemon

import (
	"net/netip"
	"time"

	"github.com/grantcostello/mynet/internal/client"
	"github.com/grantcostello/mynet/internal/control"
	"github.com/grantcostello/mynet/internal/wgbind"
	"github.com/grantcostello/mynet/internal/wgengine"
)

// Diagnose returns daemon-owned connectivity state (no extra WebSocket).
func (n *Node) Diagnose(peerName string) (client.Diagnostics, error) {
	n.mu.Lock()
	rt := n.rt
	running := n.running
	engine := n.engine
	bind := n.bind
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
			if d.ICEState == "" || d.ICEState == "idle" {
				if p.ICEConnState != "" {
					d.ICEState = p.ICEConnState
				} else {
					d.ICEState = p.State
				}
			}
			if p.Transport != "" {
				d.Connection = p.Transport
			}
			d.TURNUsed = p.TURNUsed
			if d.LocalType == "" {
				d.LocalType = p.LocalType
			}
			if d.RemoteType == "" {
				d.RemoteType = p.RemoteType
			}
			if d.LocalAddr == "" {
				d.LocalAddr = p.LocalAddr
			}
			if d.RemoteAddr == "" {
				d.RemoteAddr = p.RemoteAddr
			}
			if d.ICESessionID == "" {
				d.ICESessionID = p.ICESessionID
			}
			if bind != nil && p.VPNIP != "" {
				mergeBindStats(&d, bind, netip.MustParseAddr(p.VPNIP))
			}
			if engine != nil && p.WGPublicKey != "" {
				mergeWGStats(&d, engine, p.WGPublicKey)
			}
		}
	}
	return d, nil
}

func mergeBindStats(d *client.Diagnostics, bind *wgbind.ICEBind, vpnIP netip.Addr) {
	st := bind.Stats()
	for _, p := range st.Peers {
		if p.VPNIP != vpnIP {
			continue
		}
		d.BindHasActiveConn = p.HasActiveConn
		d.BindSendPackets = p.SendPackets
		d.BindSendBytes = p.SendBytes
		d.BindSendNoTransport = p.SendNoTransport
		d.BindSendErrors = p.SendErrors
		d.BindRecvPackets = p.RecvPackets
		d.BindRecvBytes = p.RecvBytes
		d.BindRecvDropped = p.RecvDropped
		return
	}
}

func mergeWGStats(d *client.Diagnostics, engine *wgengine.Engine, pubKey string) {
	st, ok, err := engine.PeerStatsByPublicKey(pubKey)
	if err != nil || !ok {
		return
	}
	d.WGPeerFingerprint = st.PublicKeyFingerprint
	d.WGAllowedIPs = append([]string(nil), st.AllowedIPs...)
	if !st.LastHandshake.IsZero() {
		d.WGLastHandshake = st.LastHandshake.UTC().Format(time.RFC3339)
	}
	d.WGTxBytes = st.TxBytes
	d.WGRxBytes = st.RxBytes
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
