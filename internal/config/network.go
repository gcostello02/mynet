package config

import (
	"net"
	"strings"
)

// NetworkConfig controls interface/CIDR exclusions for independent connectivity tests.
type NetworkConfig struct {
	// IndependentMode: do not use Tailscale CGNAT host candidates or auto-enable tailnet ICE from a 100.x server URL.
	IndependentMode bool `json:"independent_mode,omitempty"`
	// ExcludeCandidateCIDRs: host candidate IPs in these ranges are not gathered (e.g. 100.64.0.0/10).
	ExcludeCandidateCIDRs []string `json:"exclude_candidate_cidrs,omitempty"`
	// ExcludeInterfaces: OS interface names to skip for ICE gathering (e.g. tailscale0).
	ExcludeInterfaces []string `json:"exclude_interfaces,omitempty"`
}

// DefaultExcludeTailscaleCIDR is the Tailscale CGNAT range.
const DefaultExcludeTailscaleCIDR = "100.64.0.0/10"

// EffectiveExcludeCIDRs returns CIDR strings used for host-candidate filtering.
func (c ClientConfig) EffectiveExcludeCIDRs() []string {
	if c.Network.IndependentMode {
		if len(c.Network.ExcludeCandidateCIDRs) > 0 {
			return mergeStrings(DefaultIndependentExcludeCIDRs, c.Network.ExcludeCandidateCIDRs)
		}
		return append([]string(nil), DefaultIndependentExcludeCIDRs...)
	}
	if len(c.Network.ExcludeCandidateCIDRs) > 0 {
		return append([]string(nil), c.Network.ExcludeCandidateCIDRs...)
	}
	if !c.AllowTailscaleCandidates {
		return []string{DefaultExcludeTailscaleCIDR}
	}
	return nil
}

// ICESessionAllowTailscale reports whether 100.64.0.0/10 host candidates are allowed.
func (c ClientConfig) ICESessionAllowTailscale() bool {
	if c.Network.IndependentMode {
		return false
	}
	if c.AllowTailscaleCandidates {
		return true
	}
	host := serverHostFromURL(c.ServerURL)
	return host != "" && isTailscaleCGNAT(host)
}

// ParsedExcludeCIDRs parses EffectiveExcludeCIDRs for ICE filtering.
func (c ClientConfig) ParsedExcludeCIDRs() []*net.IPNet {
	raw := c.EffectiveExcludeCIDRs()
	out := make([]*net.IPNet, 0, len(raw))
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		_, n, err := net.ParseCIDR(s)
		if err != nil {
			continue
		}
		out = append(out, n)
	}
	return out
}
