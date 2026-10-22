package client

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gcostello02/mynet/internal/config"
	"github.com/gcostello02/mynet/internal/control"
	"github.com/gcostello02/mynet/internal/control/server"
	"github.com/gcostello02/mynet/internal/invitation"
)

func TestRecoverDeviceSessionAfterInvalidBearer(t *testing.T) {
	invitation.AllowHTTPForTest = true
	control.AllowInsecureHTTPForTest = true
	defer func() {
		invitation.AllowHTTPForTest = false
		control.AllowInsecureHTTPForTest = false
	}()

	dir := t.TempDir()
	rt, _ := enrollTestDevice(t, dir)

	rt.mu.Lock()
	rt.api.Token = "not-a-valid-bearer"
	rt.mu.Unlock()

	_, err := rt.APIClient().VPNConfig()
	if err == nil {
		t.Fatal("expected invalid bearer to fail vpn config")
	}
	if !control.IsHTTPStatus(err, 401) {
		t.Fatalf("expected 401, got %v", err)
	}

	if err := rt.RecoverDeviceSession(context.Background()); err != nil {
		t.Fatalf("recover: %v", err)
	}

	if _, err := rt.APIClient().VPNConfig(); err != nil {
		t.Fatalf("vpn config after renew: %v", err)
	}
}

func TestRecoverDeviceSessionAfterTokenExpiry(t *testing.T) {
	invitation.AllowHTTPForTest = true
	control.AllowInsecureHTTPForTest = true
	oldTTL := server.SessionTokenTTL
	server.SessionTokenTTL = 2 * time.Second
	defer func() {
		server.SessionTokenTTL = oldTTL
		invitation.AllowHTTPForTest = false
		control.AllowInsecureHTTPForTest = false
	}()

	dir := t.TempDir()
	rt, _ := enrollTestDevice(t, dir)
	time.Sleep(3 * time.Second)

	_, err := rt.APIClient().VPNConfig()
	if err == nil {
		t.Fatal("expected expired bearer to fail vpn config")
	}
	if !control.IsHTTPStatus(err, 401) {
		t.Fatalf("expected 401, got %v", err)
	}
	if err := rt.RecoverDeviceSession(context.Background()); err != nil {
		t.Fatalf("recover: %v", err)
	}
	if _, err := rt.APIClient().VPNConfig(); err != nil {
		t.Fatalf("vpn config after renew: %v", err)
	}
}

func TestSessionTokenNeedsRefreshBeforeExpiry(t *testing.T) {
	dir := t.TempDir()
	rt := NewRuntime(dir, nil)
	_ = config.SaveSessionToken(dir, config.SessionToken{
		Token:     "tok",
		ExpiresAt: time.Now().UTC().Add(2 * SessionRefreshLead).Format(time.RFC3339),
	})
	if rt.sessionTokenNeedsRefresh() {
		t.Fatal("should not refresh far before expiry")
	}
	_ = config.SaveSessionToken(dir, config.SessionToken{
		Token:     "tok",
		ExpiresAt: time.Now().UTC().Add(5 * time.Minute).Format(time.RFC3339),
	})
	if !rt.sessionTokenNeedsRefresh() {
		t.Fatal("should refresh within lead window")
	}
}

func TestRevokedDeviceSessionRenewalIsTerminal(t *testing.T) {
	invitation.AllowHTTPForTest = true
	control.AllowInsecureHTTPForTest = true
	defer func() {
		invitation.AllowHTTPForTest = false
		control.AllowInsecureHTTPForTest = false
	}()

	dir := t.TempDir()
	rt, srv := enrollTestDevice(t, dir)

	idPath := filepath.Join(rt.StateDir, "identity.json")
	idData, err := os.ReadFile(idPath)
	if err != nil {
		t.Fatal(err)
	}
	var idFile struct {
		DeviceID string `json:"device_id"`
	}
	if err := json.Unmarshal(idData, &idFile); err != nil {
		t.Fatal(err)
	}
	srv.Store().RevokeDevice(idFile.DeviceID)

	_, err = rt.APIClient().VPNConfig()
	if err == nil {
		t.Fatal("expected 401 after revocation epoch bump")
	}

	if err := rt.RecoverDeviceSession(context.Background()); err == nil {
		t.Fatal("expected terminal error for revoked device")
	}
	if !rt.AuthTerminal() {
		t.Fatal("expected auth terminal flag")
	}
}

func enrollTestDevice(t *testing.T, dir string) (*Runtime, *server.Server) {
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
	setup := httptest.NewRequest(http.MethodPost, "/v1/admin/setup", bytes.NewReader([]byte(
		`{"username":"admin","password":"longpassword12","network_display_name":"lab","public_server_url":"http://test"}`)))
	setup.RemoteAddr = "127.0.0.1:1"
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, setup)

	_, token, err := srv.Store().CreateInvitation("edge", 3600, 1, "admin")
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
	sd := filepath.Join(dir, "device")
	rt := NewRuntime(sd, nil)
	if err := rt.Join(inv, JoinOptions{DeviceName: "minipc"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ts.Close() })
	return rt, srv
}
