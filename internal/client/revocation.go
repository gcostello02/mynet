package client

// DropPeerForRevocation tears down cached ICE state for a peer removed from VPN config.
func (r *Runtime) DropPeerForRevocation(peerName string) {
	r.cancelPeerICEAttempt(peerName)
	r.ClearPeerLink(peerName, nil)
	r.mu.Lock()
	delete(r.peerInFlightICE, peerName)
	r.mu.Unlock()
}
