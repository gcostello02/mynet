package client

import (
	"strings"
	"testing"

	"github.com/grantcostello/mynet/internal/config"
	"github.com/grantcostello/mynet/internal/control"
)

func TestTurnHostIsTailscale(t *testing.T) {
	if !turnHostIsTailscale("turn:100.116.42.54:3478?transport=udp") {
		t.Fatal("expected tailscale turn host")
	}
	if turnHostIsTailscale("turn:75.75.87.144:3478") {
		t.Fatal("public IP should not be tailscale")
	}
}

func TestClassifyTailscaleSignaling(t *testing.T) {
	if classifyTailscaleSignaling("https://mesh.example.com") != "No" {
		t.Fatal("public hostname")
	}
	if classifyTailscaleSignaling("http://100.116.42.54:8787") != "Yes" {
		t.Fatal("100.x server")
	}
}

func TestIndependentWarningsTURNTailscale(t *testing.T) {
	r := &Runtime{
		cfg: config.ClientConfig{Network: config.NetworkConfig{IndependentMode: true}},
		serverICE: []control.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
			{URLs: []string{"turn:100.64.1.2:3478"}},
		},
	}
	w := r.independentWarnings()
	found := false
	for _, line := range w {
		if strings.Contains(line, "Tailscale address") {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings: %v", w)
	}
}
