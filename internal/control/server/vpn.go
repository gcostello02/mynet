package server

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/gcostello02/mynet/internal/control"
	"github.com/gcostello02/mynet/internal/identity"
)

// VPNConfig configures virtual IP allocation.
type VPNConfig struct {
	IPv4CIDR string // e.g. 10.77.0.0/16
}

func (s *Server) handleVPNRegister(w http.ResponseWriter, r *http.Request, deviceID string) {
	var req control.VPNRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := validateTimestamp(req.Timestamp, authTimeSkew); err != nil {
		httpError(w, http.StatusBadRequest, "invalid timestamp")
		return
	}
	keyHex := strings.ToLower(strings.TrimSpace(req.WGPublicKey))
	if len(keyHex) != 64 {
		httpError(w, http.StatusBadRequest, "wg_public_key must be 64 hex chars")
		return
	}
	if _, err := hex.DecodeString(keyHex); err != nil {
		httpError(w, http.StatusBadRequest, "invalid wg_public_key")
		return
	}
	dev, ok := s.store.GetDevice(deviceID)
	if !ok {
		httpError(w, http.StatusUnauthorized, "unknown device")
		return
	}
	pub, err := base64.StdEncoding.DecodeString(dev.PublicKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		httpError(w, http.StatusInternalServerError, "stored public key invalid")
		return
	}
	sig, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil {
		httpError(w, http.StatusBadRequest, "invalid signature encoding")
		return
	}
	msg := identity.CanonicalVPNRegisterMessage(deviceID, keyHex, req.Timestamp)
	if !ed25519.Verify(pub, msg, sig) {
		httpError(w, http.StatusUnauthorized, "signature verification failed")
		return
	}
	ip, err := s.store.AllocateVPNIPv4(deviceID, keyHex, s.vpnPool())
	if err != nil {
		httpError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"vpn_ipv4":      ip,
		"wg_public_key": keyHex,
	})
}

func (s *Server) handleVPNConfig(w http.ResponseWriter, r *http.Request, deviceID string) {
	dev, ok := s.store.GetDevice(deviceID)
	if !ok {
		httpError(w, http.StatusUnauthorized, "unknown device")
		return
	}
	if dev.VPNIPv4 == "" || dev.WGPublicKey == "" {
		httpError(w, http.StatusPreconditionFailed, "vpn not registered; POST /v1/vpn/register")
		return
	}
	peers := s.store.ListPeers(dev.NetworkID, deviceID)
	outPeers := make([]control.VPNPeerConfig, 0, len(peers))
	for _, p := range peers {
		if p.WGPublicKey == "" || p.VPNIPv4 == "" {
			continue
		}
		outPeers = append(outPeers, control.VPNPeerConfig{
			DeviceID:    p.DeviceID,
			Name:        p.Name,
			VPNIPv4:     p.VPNIPv4,
			WGPublicKey: p.WGPublicKey,
			AllowedIPs:  []string{p.VPNIPv4 + "/32"},
		})
	}
	writeJSON(w, http.StatusOK, control.VPNConfigResponse{
		NetworkID:   dev.NetworkID,
		Address:     dev.VPNIPv4 + "/32",
		WGPublicKey: dev.WGPublicKey,
		Peers:       outPeers,
	})
}

func (s *Server) vpnPool() *net.IPNet {
	cidr := s.cfg.VPN.IPv4CIDR
	if cidr == "" {
		cidr = "172.31.240.0/20"
	}
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		_, n, _ = net.ParseCIDR("172.31.240.0/20")
	}
	return n
}
