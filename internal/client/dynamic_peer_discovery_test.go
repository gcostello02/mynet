package client

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gcostello02/mynet/internal/control"
	"github.com/gcostello02/mynet/internal/control/server"
	"github.com/gcostello02/mynet/internal/invitation"
)

// Coordinator must list late-enrolled VPN peers for already-connected devices.
func TestVPNConfigPicksUpLateEnrolledPeers(t *testing.T) {
	invitation.AllowHTTPForTest = true
	control.AllowInsecureHTTPForTest = true
	defer func() {
		invitation.AllowHTTPForTest = false
		control.AllowInsecureHTTPForTest = false
	}()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "coord.db")
	srv, err := server.New(server.Config{
		ListenAddr:   ":0",
		ServerSecret: bytes.Repeat([]byte("x"), 32),
		DataPath:     dbPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	setup := httptest.NewRequest(http.MethodPost, "/v1/admin/setup", bytes.NewReader([]byte(
		`{"username":"admin","password":"longpassword12","network_display_name":"lab","public_server_url":"http://test"}`)))
	setup.RemoteAddr = "127.0.0.1:1"
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, setup)
	if rec.Code != http.StatusCreated {
		t.Fatalf("setup %d", rec.Code)
	}

	enrollAndVPN := func(name string) *Runtime {
		_, token, err := srv.Store().CreateInvitation(name, 3600, 1, "admin")
		if err != nil {
			t.Fatal(err)
		}
		meta := srv.Store().Meta().Snapshot()
		inv := invitation.Invite{
			Version:                invitation.Version,
			ServerURL:              ts.URL,
			NetworkID:              "default",
			CoordinatorFingerprint: meta.CoordinatorFingerprint,
			CoordinatorPublicKey:   meta.CoordinatorPublicKeyB64,
			EnrollmentToken:        token,
		}
		sd := filepath.Join(dir, name)
		rt := NewRuntime(sd, nil)
		if err := rt.Join(inv, JoinOptions{DeviceName: name}); err != nil {
			t.Fatalf("join %s: %v", name, err)
		}
		if err := rt.Login(""); err != nil {
			t.Fatalf("login %s: %v", name, err)
		}
		if _, err := rt.RegisterVPN(); err != nil {
			t.Fatalf("vpn register %s: %v", name, err)
		}
		return rt
	}

	rtA := enrollAndVPN("minipc")
	cfgA, err := rtA.APIClient().VPNConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgA.Peers) != 0 {
		t.Fatalf("A alone should have 0 vpn peers, got %d", len(cfgA.Peers))
	}

	rtB := enrollAndVPN("macbook")
	deadline := time.Now().Add(5 * time.Second)
	for {
		cfgA, err = rtA.APIClient().VPNConfig()
		if err != nil {
			t.Fatal(err)
		}
		if len(cfgA.Peers) == 1 && cfgA.Peers[0].Name == "macbook" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("A vpn config never picked up B: peers=%v", cfgA.Peers)
		}
		time.Sleep(50 * time.Millisecond)
	}

	rtC := enrollAndVPN("laptop")
	deadline = time.Now().Add(5 * time.Second)
	for {
		cfgA, err = rtA.APIClient().VPNConfig()
		if err != nil {
			t.Fatal(err)
		}
		if len(cfgA.Peers) == 2 {
			names := map[string]bool{cfgA.Peers[0].Name: true, cfgA.Peers[1].Name: true}
			if names["macbook"] && names["laptop"] {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("A vpn config missing C: %v", cfgA.Peers)
		}
		time.Sleep(50 * time.Millisecond)
	}

	cfgB, err := rtB.APIClient().VPNConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgB.Peers) != 2 {
		t.Fatalf("B expected minipc+laptop, got %d", len(cfgB.Peers))
	}
	_ = rtC
}
