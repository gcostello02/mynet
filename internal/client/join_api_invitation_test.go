package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gcostello02/mynet/internal/control"
	"github.com/gcostello02/mynet/internal/control/server"
	"github.com/gcostello02/mynet/internal/invitation"
)

// Regression: dashboard / admin API must return invitation compact strings that mynet join accepts.
func TestJoinFromAdminAPIInvitationField(t *testing.T) {
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
	setupReq := httptest.NewRequest(http.MethodPost, "/v1/admin/setup", bytes.NewReader([]byte(
		`{"username":"admin","password":"longpassword12","network_display_name":"lab","public_server_url":"http://test"}`)))
	setupReq.RemoteAddr = "127.0.0.1:1"
	setupRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(setupRec, setupReq)
	if setupRec.Code != http.StatusCreated {
		t.Fatalf("setup %d", setupRec.Code)
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	loginBody, _ := json.Marshal(map[string]string{"username": "admin", "password": "longpassword12"})
	loginReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/admin/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set(control.AdminClientCLIHeader, control.AdminClientCLIValue)
	loginRes, err := http.DefaultClient.Do(loginReq)
	if err != nil {
		t.Fatal(err)
	}
	defer loginRes.Body.Close()
	var loginOut map[string]string
	if err := json.NewDecoder(loginRes.Body).Decode(&loginOut); err != nil {
		t.Fatal(err)
	}
	token := loginOut["session_token"]
	if token == "" {
		t.Fatal("missing session_token")
	}

	invBody, _ := json.Marshal(map[string]any{"label": "edge", "expires_in_seconds": 3600, "max_uses": 1})
	invReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/admin/invitations", bytes.NewReader(invBody))
	invReq.Header.Set("Content-Type", "application/json")
	invReq.Header.Set("Authorization", "Bearer "+token)
	invRes, err := http.DefaultClient.Do(invReq)
	if err != nil {
		t.Fatal(err)
	}
	defer invRes.Body.Close()
	if invRes.StatusCode != http.StatusCreated {
		t.Fatalf("create invitation %d", invRes.StatusCode)
	}
	var invResp map[string]string
	if err := json.NewDecoder(invRes.Body).Decode(&invResp); err != nil {
		t.Fatal(err)
	}
	compact := invResp["invitation"]
	if compact == "" {
		t.Fatal("missing invitation field")
	}

	inv, err := invitation.Parse(compact)
	if err != nil {
		t.Fatal(err)
	}
	inv.ServerURL = ts.URL
	sd := filepath.Join(dir, "client")
	rt := NewRuntime(sd, nil)
	if err := rt.Join(inv, JoinOptions{DeviceName: "minipc"}); err != nil {
		t.Fatalf("join: %v", err)
	}

	// Second join with same invitation should fail when max_uses is 1.
	sd2 := filepath.Join(dir, "client2")
	rt2 := NewRuntime(sd2, nil)
	if err := rt2.Join(inv, JoinOptions{DeviceName: fmt.Sprintf("host%d", 2)}); err == nil {
		t.Fatal("expected second join to fail")
	}
}
