package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gcostello02/mynet/internal/control"
	"github.com/gcostello02/mynet/internal/identity"
)

func TestRevokedDeviceCannotReEnroll(t *testing.T) {
	srv := testServer(t)
	id, _ := enrollDevice(t, srv, "revoked-re")
	srv.store.RevokeDevice(id.DeviceID)
	ts := time.Now().UTC().Unix()
	msg := identity.CanonicalEnrollMessage(id.DeviceID, id.Name, ts)
	sig, _ := id.Sign(msg)
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
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestInvitationNotConsumedOnEnrollConflict(t *testing.T) {
	srv, _ := testServerDB(t, "")
	fp := adminSetup(t, srv)
	_, token, _ := srv.store.CreateInvitation("once", time.Hour, 1, "admin")

	id1, _ := identity.Generate("dup")
	enrollWithToken(t, srv, id1, token, fp)

	_, token2, _ := srv.store.CreateInvitation("second", time.Hour, 1, "admin")
	id2, _ := identity.Generate("dup")
	rec := enrollWithTokenRaw(srv, id2, token2, fp)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected name conflict, got %d %s", rec.Code, rec.Body.String())
	}
	if err := srv.store.CheckInvitation(token2); err != nil {
		t.Fatalf("invitation should remain valid after failed enroll, got %v", err)
	}
}

func enrollWithToken(t *testing.T, srv *Server, id *identity.DeviceIdentity, token, fp string) {
	rec := enrollWithTokenRaw(srv, id, token, fp)
	if rec.Code != http.StatusCreated {
		t.Fatalf("enroll %d %s", rec.Code, rec.Body.String())
	}
}

func enrollWithTokenRaw(srv *Server, id *identity.DeviceIdentity, token, fp string) *httptest.ResponseRecorder {
	ts := time.Now().UTC().Unix()
	msg := identity.CanonicalEnrollMessage(id.DeviceID, id.Name, ts)
	sig, _ := id.Sign(msg)
	body, _ := json.Marshal(control.EnrollRequest{
		DeviceID:               id.DeviceID,
		Name:                   id.Name,
		PublicKey:              id.PublicKey,
		Timestamp:              ts,
		Signature:              base64.StdEncoding.EncodeToString(sig),
		EnrollmentToken:        token,
		CoordinatorFingerprint: fp,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/enroll", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.handleEnroll(rec, req)
	return rec
}

func TestWrongCoordinatorFingerprintRejected(t *testing.T) {
	srv, _ := testServerDB(t, "")
	_ = adminSetup(t, srv)
	_, token, _ := srv.store.CreateInvitation("x", time.Hour, 1, "admin")
	id, _ := identity.Generate("wrong-fp")
	rec := enrollWithTokenRaw(srv, id, token, "0000000000000000000000000000000000000000000000000000000000000000")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if err := srv.store.CheckInvitation(token); err != nil {
		t.Fatalf("invitation should remain unused, got %v", err)
	}
}

func TestRevokedDeviceAuthAndSession(t *testing.T) {
	srv := testServer(t)
	id, tok := enrollDevice(t, srv, "live")
	srv.store.RevokeDevice(id.DeviceID)
	srv.hub.DisconnectDevice(id.DeviceID)

	ts := time.Now().UTC().Unix()
	msg := identity.CanonicalAuthMessage(id.DeviceID, ts)
	sig, _ := id.Sign(msg)
	body, _ := json.Marshal(control.AuthRequest{DeviceID: id.DeviceID, Timestamp: ts, Signature: base64.StdEncoding.EncodeToString(sig)})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.handleAuth(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("auth %d", rec.Code)
	}

	peersReq := httptest.NewRequest(http.MethodGet, "/v1/peers", nil)
	peersReq.Header.Set("Authorization", "Bearer "+tok)
	peersRec := httptest.NewRecorder()
	srv.withAuth(srv.handlePeers)(peersRec, peersReq)
	if peersRec.Code != http.StatusUnauthorized {
		t.Fatalf("peers %d", peersRec.Code)
	}
}

func TestAdminMutationRequiresCSRF(t *testing.T) {
	srv, _ := testServerDB(t, "")
	_ = adminSetup(t, srv)
	ac := adminLogin(t, srv)
	body, _ := json.Marshal(createInvitationRequest{MaxUses: 1})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/invitations", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: adminCookieName, Value: ac.session})
	// Missing CSRF header.
	rec := httptest.NewRecorder()
	srv.handleAdminCreateInvitation(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without csrf, got %d", rec.Code)
	}
}

func TestLegacyDisabledAfterSetupUntilEnabled(t *testing.T) {
	srv, _ := testServerDB(t, "test-enroll-secret")
	_ = adminSetup(t, srv)
	if srv.store.meta.LegacyEnrollmentEnabled {
		t.Fatal("legacy should be off after setup")
	}
	id, _ := identity.Generate("legacy-attempt")
	ts := time.Now().UTC().Unix()
	msg := identity.CanonicalEnrollMessage(id.DeviceID, id.Name, ts)
	sig, _ := id.Sign(msg)
	body, _ := json.Marshal(control.EnrollRequest{
		DeviceID: id.DeviceID, Name: id.Name, PublicKey: id.PublicKey,
		Timestamp: ts, Signature: base64.StdEncoding.EncodeToString(sig),
		EnrollmentSecret: "test-enroll-secret",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/enroll", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.handleEnroll(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("legacy enroll should fail, got %d", rec.Code)
	}
}
