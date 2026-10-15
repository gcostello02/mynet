package config

import (
	"net"
	"testing"
)

func TestEffectiveExcludeInterfacesIndependent(t *testing.T) {
	cfg := ClientConfig{Network: NetworkConfig{IndependentMode: true}}
	patterns := cfg.EffectiveExcludeInterfaces()
	if len(patterns) < len(DefaultIndependentExcludeInterfaces) {
		t.Fatalf("expected defaults merged, got %v", patterns)
	}
	found := false
	for _, p := range patterns {
		if p == "docker*" {
			found = true
		}
	}
	if !found {
		t.Fatal("docker* missing")
	}
}

func TestEffectiveExcludeCIDRsIndependent(t *testing.T) {
	cfg := ClientConfig{Network: NetworkConfig{IndependentMode: true}}
	cidrs := cfg.EffectiveExcludeCIDRs()
	if len(cidrs) < 3 {
		t.Fatalf("expected tailscale+loopback+docker range, got %v", cidrs)
	}
}

func TestEffectiveExcludeCIDRsLANRetained(t *testing.T) {
	cfg := ClientConfig{Network: NetworkConfig{IndependentMode: true}}
	nets := cfg.ParsedExcludeCIDRs()
	allow := func(ip string) bool {
		p := net.ParseIP(ip)
		for _, n := range nets {
			if n.Contains(p) {
				return false
			}
		}
		return true
	}
	if !allow("10.20.30.40") || !allow("10.0.0.50") {
		t.Fatal("physical LAN should remain allowed")
	}
	if allow("172.18.0.1") || allow("100.125.0.1") {
		t.Fatal("docker/tailscale should be blocked")
	}
}
