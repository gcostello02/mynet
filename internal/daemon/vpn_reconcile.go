package daemon

import (
	"context"
	"net/netip"
	"time"

	"github.com/gcostello02/mynet/internal/control"
)

// VPNReconcileInterval is how often the daemon refreshes authoritative peer config.
const VPNReconcileInterval = 45 * time.Second

func (n *Node) startVPNReconcile(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(VPNReconcileInterval)
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
	if !n.reconcileMu.TryLock() {
		return
	}
	defer n.reconcileMu.Unlock()

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
			if n.Logger != nil {
				n.Logger.Info("vpn config refresh denied; clearing device session")
			}
			_ = rt.ClearDeviceSession()
		} else if n.Logger != nil {
			n.Logger.Debug("vpn config refresh failed; keeping existing peers", "err", err)
		}
		return
	}

	want := make(map[string]control.VPNPeerConfig)
	for _, p := range cfg.Peers {
		if p.Name == "" || p.WGPublicKey == "" || p.VPNIPv4 == "" || len(p.WGPublicKey) != 64 {
			continue
		}
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
			var peerIP netip.Addr
			n.mu.Lock()
			if st, ok := n.peers[name]; ok && st.VPNIP != "" {
				peerIP, _ = netip.ParseAddr(st.VPNIP)
			}
			bind := n.bind
			n.mu.Unlock()
			n.stopVPNPeerWorker(name)
			_ = engine.RemovePeer(pub)
			rt.DropPeerForRevocation(name)
			if bind != nil && peerIP.IsValid() {
				bind.RemovePeer(peerIP)
			}
			n.mu.Lock()
			delete(n.vpnPeerKeys, name)
			delete(n.peers, name)
			n.mu.Unlock()
			if n.Logger != nil {
				n.Logger.Info("removed vpn peer not in coordinator config", "peer", name)
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
		if err := engine.SetPeer(p.WGPublicKey, peerIP, p.VPNIPv4+"/32"); err != nil {
			if n.Logger != nil {
				n.Logger.Debug("reconcile add peer", "peer", p.Name, "err", err)
			}
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
	return control.IsHTTPStatus(err, 401) || control.IsHTTPStatus(err, 403)
}
