package config

import "testing"

func TestEffectiveICEURLs(t *testing.T) {
	c := ClientConfig{
		STUNURLs: []string{"stun:a:1"},
		TURNURLs: []string{"turn:b:2"},
	}
	got := c.EffectiveICEURLs()
	if len(got) != 2 || got[0] != "stun:a:1" || got[1] != "turn:b:2" {
		t.Fatalf("merge: %v", got)
	}
	c.ICEURLs = []string{"stun:only:1"}
	got = c.EffectiveICEURLs()
	if len(got) != 1 || got[0] != "stun:only:1" {
		t.Fatalf("ice_urls override: %v", got)
	}
}

func TestICESessionAllowTailscale(t *testing.T) {
	c := ClientConfig{ServerURL: "http://100.64.1.1:8787"}
	if !c.ICESessionAllowTailscale() {
		t.Fatal("expected auto tailscale for 100.x server")
	}
	c.ServerURL = "http://10.0.0.50:8787"
	if c.ICESessionAllowTailscale() {
		t.Fatal("expected LAN server without explicit flag")
	}
	c.AllowTailscaleCandidates = true
	if !c.ICESessionAllowTailscale() {
		t.Fatal("expected explicit allow")
	}
	c.Network.IndependentMode = true
	if c.ICESessionAllowTailscale() {
		t.Fatal("independent mode must block tailnet ICE")
	}
}
