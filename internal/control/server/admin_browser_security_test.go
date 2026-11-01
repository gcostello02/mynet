package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gcostello02/mynet/internal/control"
)

func adminCookieRequest(method, path string, body []byte, ac adminCookies) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.AddCookie(&http.Cookie{Name: adminCookieName, Value: ac.session})
	req.AddCookie(&http.Cookie{Name: adminCSRFCookie, Value: ac.csrf})
	req.Header.Set(adminCSRFHeader, ac.csrf)
	return req
}

func TestAdminUnauthenticatedEndpoints(t *testing.T) {
	srv, _ := testServerDB(t, "")
	_ = adminSetup(t, srv)
	h := srv.Handler()

	for _, path := range []string{"/v1/admin/status", "/v1/admin/devices", "/v1/admin/invitations"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s: expected 401, got %d", path, rec.Code)
		}
	}
}

func TestAdminMutationRequiresCSRF(t *testing.T) {
	srv, _ := testServerDB(t, "")
	_ = adminSetup(t, srv)
	ac := adminLogin(t, srv)
	body, _ := json.Marshal(createInvitationRequest{MaxUses: 1})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/invitations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: adminCookieName, Value: ac.session})
	rec := httptest.NewRecorder()
	srv.handleAdminCreateInvitation(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without csrf, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestAdminMutationRejectsWrongCSRF(t *testing.T) {
	srv, _ := testServerDB(t, "")
	_ = adminSetup(t, srv)
	ac := adminLogin(t, srv)
	body, _ := json.Marshal(createInvitationRequest{MaxUses: 1})
	req := adminCookieRequest(http.MethodPost, "/v1/admin/invitations", body, ac)
	req.Header.Set(adminCSRFHeader, "wrong-token")
	rec := httptest.NewRecorder()
	srv.handleAdminCreateInvitation(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestAdminMutationRejectsCrossSessionCSRF(t *testing.T) {
	srv, _ := testServerDB(t, "")
	_ = adminSetup(t, srv)
	ac1 := adminLogin(t, srv)
	ac2 := adminLogin(t, srv)
	body, _ := json.Marshal(createInvitationRequest{MaxUses: 1})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/invitations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: adminCookieName, Value: ac1.session})
	req.AddCookie(&http.Cookie{Name: adminCSRFCookie, Value: ac2.csrf})
	req.Header.Set(adminCSRFHeader, ac2.csrf)
	rec := httptest.NewRecorder()
	srv.handleAdminCreateInvitation(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-session csrf, got %d", rec.Code)
	}
}

func TestAdminBearerMutationWithoutCSRF(t *testing.T) {
	srv, _ := testServerDB(t, "")
	_ = adminSetup(t, srv)
	ac := adminLogin(t, srv)
	body, _ := json.Marshal(createInvitationRequest{Label: "bearer", MaxUses: 1})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/invitations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ac.session)
	rec := httptest.NewRecorder()
	srv.handleAdminCreateInvitation(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("bearer create: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAdminMutationRejectsForeignOrigin(t *testing.T) {
	srv, _ := testServerDB(t, "")
	_ = adminSetup(t, srv)
	ac := adminLogin(t, srv)
	body, _ := json.Marshal(createInvitationRequest{MaxUses: 1})
	req := adminCookieRequest(http.MethodPost, "/v1/admin/invitations", body, ac)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	srv.handleAdminCreateInvitation(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for foreign origin, got %d", rec.Code)
	}
}

func TestAdminLogoutRequiresCSRF(t *testing.T) {
	srv, _ := testServerDB(t, "")
	_ = adminSetup(t, srv)
	ac := adminLogin(t, srv)
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/logout", nil)
	req.AddCookie(&http.Cookie{Name: adminCookieName, Value: ac.session})
	rec := httptest.NewRecorder()
	srv.handleAdminLogout(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("logout without csrf: expected 403, got %d", rec.Code)
	}
}

func TestAdminLoginOmitsSessionTokenForBrowser(t *testing.T) {
	srv, _ := testServerDB(t, "")
	_ = adminSetup(t, srv)
	body, _ := json.Marshal(adminLoginRequest{Username: "adminuser", Password: "longpassword12"})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.handleAdminLogin(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login %d", rec.Code)
	}
	var m map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	if m["session_token"] != "" {
		t.Fatal("browser login must not include session_token in JSON")
	}
}

func TestAdminLoginIncludesSessionTokenForCLI(t *testing.T) {
	srv, _ := testServerDB(t, "")
	_ = adminSetup(t, srv)
	body, _ := json.Marshal(adminLoginRequest{Username: "adminuser", Password: "longpassword12"})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/login", bytes.NewReader(body))
	req.Header.Set(control.AdminClientCLIHeader, control.AdminClientCLIValue)
	rec := httptest.NewRecorder()
	srv.handleAdminLogin(rec, req)
	var m map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	if m["session_token"] == "" {
		t.Fatal("cli login should include session_token")
	}
}

func TestAdminResponsesNoStore(t *testing.T) {
	srv, _ := testServerDB(t, "")
	_ = adminSetup(t, srv)
	h := srv.Handler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/admin/status", nil))
	if rec.Header().Get("Cache-Control") == "" {
		t.Fatal("expected Cache-Control on admin API")
	}
}
