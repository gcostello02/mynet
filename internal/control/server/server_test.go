package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/grantcostello/mynet/internal/control"
	"github.com/grantcostello/mynet/internal/identity"
)

func testServer(t *testing.T) *Server {
	srv, err := New(Config{
		ListenAddr:       ":0",
		ServerSecret:     bytes.Repeat([]byte("x"), 32),
		EnrollmentSecret: "test-enroll-secret",
		NetworkID:        "testnet",
	})
	if err != nil {
		t.Fatal(err)
	}
	return srv
}

func enrollDevice(t *testing.T, srv *Server, name string) (*identity.DeviceIdentity, string) {
	id, err := identity.Generate(name)
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Now().UTC().Unix()
	msg := identity.CanonicalEnrollMessage(id.DeviceID, id.Name, ts)
	sig, err := id.Sign(msg)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(control.EnrollRequest{
		DeviceID:         id.DeviceID,
		Name:             id.Name,
		PublicKey:        id.PublicKey,
		Timestamp:        ts,
		Signature:        base64.StdEncoding.EncodeToString(sig),
		EnrollmentSecret: "test-enroll-secret",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/enroll", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.handleEnroll(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("enroll status %d body %s", rec.Code, rec.Body.String())
	}
	token := authDevice(t, srv, id)
	return id, token
}

func authDevice(t *testing.T, srv *Server, id *identity.DeviceIdentity) string {
	ts := time.Now().UTC().Unix()
	msg := identity.CanonicalAuthMessage(id.DeviceID, ts)
	sig, err := id.Sign(msg)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(control.AuthRequest{
		DeviceID:  id.DeviceID,
		Timestamp: ts,
		Signature: base64.StdEncoding.EncodeToString(sig),
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.handleAuth(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("auth status %d", rec.Code)
	}
	var resp control.AuthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	return resp.Token
}

func TestEnrollAuthPeersUnauthorized(t *testing.T) {
	srv := testServer(t)
	a, tokA := enrollDevice(t, srv, "alpha")
	_, tokB := enrollDevice(t, srv, "bravo")

	req := httptest.NewRequest(http.MethodGet, "/v1/peers", nil)
	req.Header.Set("Authorization", "Bearer "+tokA)
	rec := httptest.NewRecorder()
	srv.withAuth(srv.handlePeers)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("peers %d", rec.Code)
	}
	var peers []control.PeerInfo
	if err := json.NewDecoder(rec.Body).Decode(&peers); err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 || peers[0].Name != "bravo" {
		t.Fatalf("unexpected peers %#v", peers)
	}

	badReq := httptest.NewRequest(http.MethodGet, "/v1/peers", nil)
	badReq.Header.Set("Authorization", "Bearer invalid")
	badRec := httptest.NewRecorder()
	srv.withAuth(srv.handlePeers)(badRec, badReq)
	if badRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", badRec.Code)
	}

	// Unknown device signature should fail auth.
	rogue, err := identity.Generate("rogue")
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Now().UTC().Unix()
	msg := identity.CanonicalAuthMessage(rogue.DeviceID, ts)
	sig, _ := rogue.Sign(msg)
	body, _ := json.Marshal(control.AuthRequest{
		DeviceID:  rogue.DeviceID,
		Timestamp: ts,
		Signature: base64.StdEncoding.EncodeToString(sig),
	})
	req2 := httptest.NewRequest(http.MethodPost, "/v1/auth", bytes.NewReader(body))
	rec2 := httptest.NewRecorder()
	srv.handleAuth(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("rogue auth should fail, got %d", rec2.Code)
	}

	_ = a
	_ = tokB
}

func TestICEServers(t *testing.T) {
	srv, err := New(Config{
		ListenAddr:       ":0",
		ServerSecret:     bytes.Repeat([]byte("x"), 32),
		EnrollmentSecret: "test-enroll-secret",
		NetworkID:        "testnet",
		ICE: ICEConfig{
			STUNURLs:         []string{"stun:stun.test:3478"},
			TURNURLs:         []string{"turn:turn.test:3478?transport=udp"},
			TURNSharedSecret: []byte("turn-secret"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, tok := enrollDevice(t, srv, "ice-a")
	req := httptest.NewRequest(http.MethodGet, "/v1/ice-servers", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	srv.withAuth(srv.handleICEServersAuth)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ice-servers %d %s", rec.Code, rec.Body.String())
	}
	var resp control.ICEServersResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.ICEServers) != 2 {
		t.Fatalf("expected stun+turn, got %#v", resp.ICEServers)
	}
	if resp.ICEServers[1].Username == "" || resp.ICEServers[1].Credential == "" {
		t.Fatalf("expected turn creds %#v", resp.ICEServers[1])
	}
}

func TestRevokeDevice(t *testing.T) {
	srv := testServer(t)
	id, _ := enrollDevice(t, srv, "revoke-me")
	if !srv.Store().RevokeDevice(id.DeviceID) {
		t.Fatal("revoke failed")
	}
	_, ok := srv.Store().GetDevice(id.DeviceID)
	if ok {
		t.Fatal("revoked device should not be returned")
	}
}
