package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gcostello02/mynet/internal/control"
)

func adminPasswordChange(t *testing.T, srv *Server, ac adminCookies, current, newPass string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(adminPasswordChangeRequest{CurrentPassword: current, NewPassword: newPass})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/password", bytes.NewReader(body))
	req.Header.Set("Origin", "https://coordinator.test")
	req.Header.Set(control.AdminClientCLIHeader, control.AdminClientCLIValue)
	req.Header.Set("Authorization", "Bearer "+ac.session)
	rec := httptest.NewRecorder()
	srv.handleAdminPasswordChange(rec, req)
	return rec
}

func TestAdminPasswordChangeSuccessInvalidatesSessions(t *testing.T) {
	srv, _ := testServerDB(t, "")
	adminSetup(t, srv)
	ac := adminLogin(t, srv)

	rec := adminPasswordChange(t, srv, ac, "longpassword12", "anotherpassword12")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("change %d %s", rec.Code, rec.Body.String())
	}

	admin := srv.store.admins[srv.store.adminByUser["adminuser"]]
	if !strings.HasPrefix(admin.PasswordHash, "argon2id$") {
		t.Fatalf("expected argon2id hash, got prefix %q", admin.PasswordHash[:12])
	}
	if verifyAdminPassword("longpassword12", admin.PasswordHash) {
		t.Fatal("old password should not verify")
	}
	if !verifyAdminPassword("anotherpassword12", admin.PasswordHash) {
		t.Fatal("new password should verify")
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/v1/admin/status", nil)
	statusReq.AddCookie(&http.Cookie{Name: adminCookieName, Value: ac.session})
	statusRec := httptest.NewRecorder()
	srv.handleAdminStatus(statusRec, statusReq)
	if statusRec.Code != http.StatusUnauthorized {
		t.Fatalf("old session should be invalid, got %d", statusRec.Code)
	}

	ac2 := adminLoginPassword(t, srv, "anotherpassword12")
	if ac2.session == ac.session {
		t.Fatal("expected new session id after re-login")
	}
}

func adminLoginPassword(t *testing.T, srv *Server, password string) adminCookies {
	body, _ := json.Marshal(adminLoginRequest{Username: "adminuser", Password: password})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.handleAdminLogin(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login %d %s", rec.Code, rec.Body.String())
	}
	var out adminCookies
	for _, c := range rec.Result().Cookies() {
		if c.Name == adminCookieName {
			out.session = c.Value
		}
		if c.Name == adminCSRFCookie {
			out.csrf = c.Value
		}
	}
	if out.session == "" || out.csrf == "" {
		t.Fatal("missing session cookies")
	}
	return out
}

func TestAdminPasswordChangeWrongCurrent(t *testing.T) {
	srv, _ := testServerDB(t, "")
	adminSetup(t, srv)
	ac := adminLogin(t, srv)

	rec := adminPasswordChange(t, srv, ac, "wrongpassword1", "anotherpassword12")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestAdminPasswordChangeWeakNew(t *testing.T) {
	srv, _ := testServerDB(t, "")
	adminSetup(t, srv)
	ac := adminLogin(t, srv)

	rec := adminPasswordChange(t, srv, ac, "longpassword12", "short")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestAdminPasswordChangeBrowserClearsCookies(t *testing.T) {
	srv, _ := testServerDB(t, "")
	adminSetup(t, srv)
	ac := adminLogin(t, srv)

	body, _ := json.Marshal(adminPasswordChangeRequest{CurrentPassword: "longpassword12", NewPassword: "anotherpassword12"})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/password", bytes.NewReader(body))
	req.Host = "coordinator.test"
	req.Header.Set("Origin", "https://coordinator.test")
	req.AddCookie(&http.Cookie{Name: adminCookieName, Value: ac.session})
	req.AddCookie(&http.Cookie{Name: adminCSRFCookie, Value: ac.csrf})
	req.Header.Set(adminCSRFHeader, ac.csrf)
	rec := httptest.NewRecorder()
	srv.handleAdminPasswordChange(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("change %d %s", rec.Code, rec.Body.String())
	}
	var cleared bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == adminCookieName && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatal("expected session cookie cleared")
	}
}
