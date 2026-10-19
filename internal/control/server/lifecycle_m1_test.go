package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/grantcostello/mynet/internal/identity"
)

func TestCoordinatorLifecycleRestart(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "lifecycle.db")

	srv1, err := New(Config{
		ListenAddr:   ":0",
		ServerSecret: bytes.Repeat([]byte("s"), 32),
		DataPath:     dbPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	fp := adminSetup(t, srv1)
	ac := adminLogin(t, srv1)

	invBody, _ := json.Marshal(createInvitationRequest{MaxUses: 1, ExpiresIn: 3600})
	invReq := httptest.NewRequest(http.MethodPost, "/v1/admin/invitations", bytes.NewReader(invBody))
	invReq.AddCookie(&http.Cookie{Name: adminCookieName, Value: ac.session})
	invReq.AddCookie(&http.Cookie{Name: adminCSRFCookie, Value: ac.csrf})
	invReq.Header.Set(adminCSRFHeader, ac.csrf)
	invRec := httptest.NewRecorder()
	srv1.handleAdminCreateInvitation(invRec, invReq)
	var invResp map[string]string
	_ = json.NewDecoder(invRec.Body).Decode(&invResp)
	token := invResp["enrollment_token"]
	fp1 := srv1.coordinatorIdentity.Fingerprint

	srv2, err := New(Config{
		ListenAddr:   ":0",
		ServerSecret: bytes.Repeat([]byte("s"), 32),
		DataPath:     dbPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if srv2.coordinatorIdentity.Fingerprint != fp1 || fp1 != fp {
		t.Fatalf("fingerprint changed across restart")
	}
	_ = adminLogin(t, srv2)

	id, _ := identity.Generate("post-restart")
	enrollWithToken(t, srv2, id, token, fp)

	tok := authDevice(t, srv2, id)
	srv2.store.RevokeDevice(id.DeviceID)
	srv2.hub.DisconnectDevice(id.DeviceID)

	peersReq := httptest.NewRequest(http.MethodGet, "/v1/peers", nil)
	peersReq.Header.Set("Authorization", "Bearer "+tok)
	peersRec := httptest.NewRecorder()
	srv2.withAuth(srv2.handlePeers)(peersRec, peersReq)
	if peersRec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked bearer should fail, got %d", peersRec.Code)
	}
}

func TestCoordinatorProofValid(t *testing.T) {
	srv, _ := testServerDB(t, "")
	fp := adminSetup(t, srv)
	ts := time.Now().UTC().Unix()
	body, _ := json.Marshal(networkProofRequest{Timestamp: ts})
	req := httptest.NewRequest(http.MethodPost, "/v1/network/proof", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.handleNetworkProof(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("proof %d %s", rec.Code, rec.Body.String())
	}
	var resp networkProofResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if !VerifyCoordinatorProof(resp.NetworkID, fp, resp.CoordinatorPublicKey, ts, resp.Signature, authTimeSkew) {
		t.Fatal("proof verification failed")
	}
}

func TestCoordinatorKeyMissingFailsStart(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "missing-key.db")
	srv, err := New(Config{ListenAddr: ":0", ServerSecret: bytes.Repeat([]byte("s"), 32), DataPath: dbPath})
	if err != nil {
		t.Fatal(err)
	}
	adminSetup(t, srv)
	keyPath := coordinatorKeyPath(dbPath)
	if err := os.Remove(keyPath); err != nil {
		t.Fatal(err)
	}
	_, err = New(Config{ListenAddr: ":0", ServerSecret: bytes.Repeat([]byte("s"), 32), DataPath: dbPath})
	if err == nil {
		t.Fatal("expected error when coordinator key missing")
	}
}

func TestCoordinatorKeyCorruptFailsStart(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "corrupt-key.db")
	srv, err := New(Config{ListenAddr: ":0", ServerSecret: bytes.Repeat([]byte("s"), 32), DataPath: dbPath})
	if err != nil {
		t.Fatal(err)
	}
	adminSetup(t, srv)
	if err := os.WriteFile(coordinatorKeyPath(dbPath), []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = New(Config{ListenAddr: ":0", ServerSecret: bytes.Repeat([]byte("s"), 32), DataPath: dbPath})
	if err == nil {
		t.Fatal("expected error for corrupt coordinator key")
	}
}
