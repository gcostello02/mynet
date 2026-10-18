package wgengine

import (
	"encoding/hex"
	"fmt"

	"golang.zx2c4.com/wireguard/device"
)

// NudgeHandshake asks wireguard-go to send a handshake initiation for a configured peer.
// Safe to call after ICE transport attach; no-op if the peer is unknown.
func (e *Engine) NudgeHandshake(publicKeyHex string) error {
	if e == nil || e.dev == nil {
		return fmt.Errorf("wireguard engine not ready")
	}
	raw, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return fmt.Errorf("invalid public key: %w", err)
	}
	if len(raw) != device.NoisePublicKeySize {
		return fmt.Errorf("invalid public key length %d", len(raw))
	}
	var pk device.NoisePublicKey
	copy(pk[:], raw)
	peer := e.dev.LookupPeer(pk)
	if peer == nil {
		return fmt.Errorf("wireguard peer not configured")
	}
	return peer.SendHandshakeInitiation(false)
}
