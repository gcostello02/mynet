package daemon

import (
	"context"
	"net/netip"
	"strings"
	"time"

	"github.com/gcostello02/mynet/internal/control"
)

const vpnReconcileInterval = 45 * time.Second

func (n *Node) startVPNReconcile(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(vpnReconcileInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				n.reconcileVPNPeers()
			}
		}
	}()
}

func (n *Node) reconcileVPNPeers() {
	n.mu.Lock()
	rt := n.rt
	engine := n.engine
	tunMgr := n.tunMgr
	if rt == nil || engine == nil || !n.running {
		n.mu.Unlock()
		return
	}
	n.mu.Unlock()

	cfg, err := rt.APIClient().VPNConfig()
	if err != nil {
		if isAuthAPIError(err) {
			n.Logger.Info("vpn config refresh denied; clearing device session", "err", err)
			_ = rt.ClearDeviceSession()
		}
		return
	}

	want := make(map[string]control.VPNPeerConfig, len(cfg.Peers))
	for _, p := range cfg.Peers {
		want[p.Name] = p
	}

	n.mu.Lock()
	current := make(map[string]string, len(n.vpnPeerKeys))
	for name, key := range n.vpnPeerKeys {
		current[name] = key
	}
	n.mu.Unlock()

	for name, pub := range current {
		if _, ok := want[name]; !ok {
			_ = engine.RemovePeer(pub)
			rt.DropPeerForRevocation(name)
			n.mu.Lock()
			delete(n.vpnPeerKeys, name)
			delete(n.peers, name)
			n.mu.Unlock()
			if n.Logger != nil {
				n.Logger.Info("removed revoked or unknown vpn peer", "peer", name)
			}
		}
	}

	for _, p := range want {
		if _, ok := current[p.Name]; ok {
			continue
		}
		peerIP := netip.MustParseAddr(p.VPNIPv4)
		if tunMgr != nil {
			_ = tunMgr.AddPeerRoute(peerIP)
		}
		if err := engine.SetPeer(p.WGPublicKey, peerIP, p.VPNIPv4+"/32"); err != nil && n.Logger != nil {
			n.Logger.Debug("reconcile add peer", "peer", p.Name, "err", err)
			continue
		}
		n.mu.Lock()
		n.vpnPeerKeys[p.Name] = p.WGPublicKey
		n.mu.Unlock()
	}
}

func isAuthAPIError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "401") || strings.Contains(s, "403") || strings.Contains(s, "revoked")
}
