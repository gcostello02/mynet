package client

// syncPeerPresence invalidates stale VPN transports when coordinator presence flips.
func (r *Runtime) syncPeerPresence(peerName string) {
	if peerName == "" || r.api == nil || r.api.Token == "" {
		return
	}
	peers, err := r.api.Peers()
	if err != nil {
		return
	}
	for _, p := range peers {
		if p.Name == peerName {
			r.notePeerOnlineState(peerName, p.Online)
			return
		}
	}
}

func (r *Runtime) notePeerOnlineState(peerName string, online bool) {
	r.mu.Lock()
	prev, had := r.peerOnlineLast[peerName]
	if r.peerOnlineLast == nil {
		r.peerOnlineLast = make(map[string]bool)
	}
	r.peerOnlineLast[peerName] = online
	r.mu.Unlock()
	if !r.vpnMode {
		return
	}
	if had && prev && !online {
		r.invalidatePeerLinksForPresence(peerName, "coord_offline")
	}
	if had && !prev && online {
		r.invalidatePeerLinksForPresence(peerName, "coord_online")
	}
}

func (r *Runtime) invalidatePeerLinksForPresence(peerName, reason string) {
	r.mu.Lock()
	l, ok := r.peerLinks[peerName]
	if !ok || l == nil {
		r.mu.Unlock()
		return
	}
	r.supersedeVPNPeerLinkLocked(peerName, l, reason)
	r.mu.Unlock()
}
