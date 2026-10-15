package config

import (
	"testing"

	"github.com/grantcostello/mynet/internal/control"
)

func TestFilterICEServersDropsTailscaleTURN(t *testing.T) {
	cfg := ClientConfig{Network: NetworkConfig{IndependentMode: true}}
	servers := []control.ICEServer{{
		URLs:       []string{"stun:stun.l.google.com:19302", "turn:100.116.42.54:3478?transport=udp"},
		Username:   "u",
		Credential: "p",
	}}
	out := FilterICEServers(cfg, servers)
	if len(out) != 1 {
		t.Fatalf("expected one server entry, got %d", len(out))
	}
	if len(out[0].URLs) != 1 || out[0].URLs[0] != "stun:stun.l.google.com:19302" {
		t.Fatalf("turn on 100.x should be removed, got %v", out[0].URLs)
	}
}

func TestFilterICEServersKeepsPublicTURNHostname(t *testing.T) {
	cfg := ClientConfig{Network: NetworkConfig{IndependentMode: true}}
	servers := []control.ICEServer{{
		URLs: []string{"turn:turn.example.com:3478?transport=udp"},
	}}
	out := FilterICEServers(cfg, servers)
	if len(out) != 1 || len(out[0].URLs) != 1 {
		t.Fatalf("public hostname TURN should remain, got %#v", out)
	}
}

func TestICEURLHost(t *testing.T) {
	if iceURLHost("turn:100.116.42.54:3478?transport=udp") != "100.116.42.54" {
		t.Fatal("parse turn host")
	}
}
