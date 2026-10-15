package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gcostello02/mynet/internal/config"
	"github.com/gcostello02/mynet/internal/identity"
	"github.com/gcostello02/mynet/internal/wgkeys"
)

func TestRegisterVPN(t *testing.T) {
	dir := t.TempDir()
	id, err := identity.Generate("testdev")
	if err != nil {
		t.Fatal(err)
	}
	if err := identity.Save(config.IdentityPath(dir), id); err != nil {
		t.Fatal(err)
	}
	if err := config.SaveClientConfig(dir, config.ClientConfig{ServerURL: "http://127.0.0.1:1"}); err != nil {
		t.Fatal(err)
	}
	if err := config.SaveSessionToken(dir, config.SessionToken{Token: "test-token"}); err != nil {
		t.Fatal(err)
	}
	wg, err := wgkeys.LoadOrCreate(dir)
	if err != nil {
		t.Fatal(err)
	}
	wgHex, err := wg.PublicKeyHex()
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/vpn/register":
			if r.Header.Get("Authorization") != "Bearer test-token" {
				http.Error(w, "unauth", http.StatusUnauthorized)
				return
			}
			writeJSON(w, map[string]string{"vpn_ipv4": "10.77.0.42", "wg_public_key": wgHex})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	cfg, _ := config.LoadClientConfig(dir)
	cfg.ServerURL = srv.URL
	if err := config.SaveClientConfig(dir, cfg); err != nil {
		t.Fatal(err)
	}

	rt := NewRuntime(dir, nil)
	ip, err := rt.RegisterVPN()
	if err != nil {
		t.Fatal(err)
	}
	if ip != "10.77.0.42" {
		t.Fatalf("ip %q", ip)
	}
	loaded, err := wgkeys.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.VPNIPv4 != "10.77.0.42" {
		t.Fatalf("wg.json vpn_ipv4 %q", loaded.VPNIPv4)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
