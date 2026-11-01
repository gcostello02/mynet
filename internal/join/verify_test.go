package join

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gcostello02/mynet/internal/control"
	"github.com/gcostello02/mynet/internal/control/server"
	"github.com/gcostello02/mynet/internal/invitation"
)

func startTestCoordinator(t *testing.T) (*httptest.Server, invitation.Invite) {
	invitation.AllowHTTPForTest = true
	control.AllowInsecureHTTPForTest = true
	dir := t.TempDir()
	srv, err := server.New(server.Config{
		ListenAddr:   ":0",
		ServerSecret: bytes.Repeat([]byte("s"), 32),
		DataPath:     filepath.Join(dir, "db.sqlite"),
	})
	if err != nil {
		t.Fatal(err)
	}
	setupReq := httptest.NewRequest(http.MethodPost, "/v1/admin/setup", bytes.NewReader([]byte(
		`{"username":"admin","password":"longpassword12","network_display_name":"lab","public_server_url":"http://test"}`)))
	setupReq.RemoteAddr = "127.0.0.1:1"
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, setupReq)
	if rec.Code != http.StatusCreated {
		t.Fatalf("setup %d", rec.Code)
	}
	ts := httptest.NewServer(srv.Handler())
	meta := srv.Store().Meta().Snapshot()
	_, token, err := srv.Store().CreateInvitation("join", 3600, 1, "admin")
	if err != nil {
		t.Fatal(err)
	}
	inv := invitation.Invite{
		Version:                invitation.Version,
		ServerURL:              ts.URL,
		NetworkID:              "default",
		CoordinatorFingerprint: meta.CoordinatorFingerprint,
		CoordinatorPublicKey:     meta.CoordinatorPublicKeyB64,
		EnrollmentToken:        token,
	}
	return ts, inv
}

func TestVerifyCoordinatorOK(t *testing.T) {
	ts, inv := startTestCoordinator(t)
	defer ts.Close()
	if err := VerifyCoordinator(inv); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyCoordinatorRejectsV1Downgrade(t *testing.T) {
	ts, inv := startTestCoordinator(t)
	defer ts.Close()
	malicious := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/network/proof" {
			meta := struct {
				NetworkID              string `json:"network_id"`
				CoordinatorFingerprint string `json:"coordinator_fingerprint"`
				CoordinatorPublicKey   string `json:"coordinator_public_key"`
				Timestamp              int64  `json:"timestamp"`
				Signature              string `json:"signature"`
				ProofVersion           int    `json:"proof_version"`
			}{
				NetworkID:              inv.NetworkID,
				CoordinatorFingerprint: inv.CoordinatorFingerprint,
				CoordinatorPublicKey:   inv.CoordinatorPublicKey,
				Timestamp:              time.Now().UTC().Unix(),
				Signature:              base64.StdEncoding.EncodeToString([]byte("invalid")),
				ProofVersion:           1,
			}
			_ = json.NewEncoder(w).Encode(meta)
			return
		}
		ts.Config.Handler.ServeHTTP(w, r)
	}))
	defer malicious.Close()
	badInv := inv
	badInv.ServerURL = malicious.URL
	err := VerifyCoordinator(badInv)
	if !errors.Is(err, ErrProofDowngrade) {
		t.Fatalf("expected downgrade error, got %v", err)
	}
}

func TestVerifyCoordinatorRejectsForgedProof(t *testing.T) {
	ts, inv := startTestCoordinator(t)
	defer ts.Close()
	_, wrongPriv, _ := ed25519.GenerateKey(nil)
	forged := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/network/proof" {
			ts.Config.Handler.ServeHTTP(w, r)
			return
		}
		var req struct {
			Timestamp int64  `json:"timestamp"`
			Challenge string `json:"challenge"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		sig := ed25519.Sign(wrongPriv, []byte("not-a-real-canonical-message"))
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"network_id":               inv.NetworkID,
			"coordinator_fingerprint":  inv.CoordinatorFingerprint,
			"coordinator_public_key":   inv.CoordinatorPublicKey,
			"timestamp":                req.Timestamp,
			"challenge":                req.Challenge,
			"signature":                base64.StdEncoding.EncodeToString(sig),
			"proof_version":            2,
		})
	}))
	defer forged.Close()
	badInv := inv
	badInv.ServerURL = forged.URL
	if err := VerifyCoordinator(badInv); err == nil {
		t.Fatal("expected invalid proof")
	}
}

func TestVerifyCoordinatorRejectsFingerprintMismatch(t *testing.T) {
	ts, inv := startTestCoordinator(t)
	defer ts.Close()
	inv.CoordinatorFingerprint = fmt.Sprintf("%064x", bytes.Repeat([]byte{1}, 32))
	if err := VerifyCoordinator(inv); err == nil {
		t.Fatal("expected fingerprint mismatch")
	}
}

func TestVerifyCoordinatorRejectsRedirectHost(t *testing.T) {
	ts, inv := startTestCoordinator(t)
	defer ts.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/network/public" {
			http.Redirect(w, r, "http://evil.example/v1/network/public", http.StatusFound)
			return
		}
		ts.Config.Handler.ServeHTTP(w, r)
	}))
	defer redirector.Close()
	inv.ServerURL = redirector.URL
	if err := VerifyCoordinator(inv); err == nil {
		t.Fatal("expected redirect block")
	}
}
