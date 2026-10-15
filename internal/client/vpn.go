package client

import (
	"fmt"

	"github.com/grantcostello/mynet/internal/config"
	"github.com/grantcostello/mynet/internal/wgkeys"
)

// RegisterVPN loads local identity and WireGuard keys, authenticates via the saved
// session, registers the WG public key with the coordination server, and persists
// the assigned virtual IPv4 in wg.json.
func (r *Runtime) RegisterVPN() (string, error) {
	if err := config.IdentityMustExist(r.StateDir); err != nil {
		return "", err
	}
	if err := r.LoadSessionOnly(); err != nil {
		return "", err
	}
	if r.api == nil || r.api.Token == "" {
		return "", fmt.Errorf("not logged in: run mynet login first")
	}
	wgID, err := wgkeys.LoadOrCreate(r.StateDir)
	if err != nil {
		return "", fmt.Errorf("wireguard keys: %w", err)
	}
	wgHex, err := wgID.PublicKeyHex()
	if err != nil {
		return "", err
	}
	resp, err := r.api.VPNRegister(r.identity, wgHex)
	if err != nil {
		return "", fmt.Errorf("vpn register: %w", err)
	}
	if resp.VPNIPv4 == "" {
		return "", fmt.Errorf("vpn register: server returned empty vpn_ipv4")
	}
	if err := wgkeys.SaveAssignedVPNIPv4(r.StateDir, resp.VPNIPv4); err != nil {
		return "", fmt.Errorf("save assigned ip: %w", err)
	}
	return resp.VPNIPv4, nil
}
