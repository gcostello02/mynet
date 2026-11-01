package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gcostello02/mynet/internal/invitation"
)

func TestAdminCreateInvitationReturnsCompactJoinPayload(t *testing.T) {
	srv, _ := testServerDB(t, "")
	adminSetup(t, srv)
	ac := adminLogin(t, srv)

	body, _ := json.Marshal(createInvitationRequest{Label: "laptop", MaxUses: 1, ExpiresIn: 3600})
	req := adminCookieRequest(http.MethodPost, "/v1/admin/invitations", body, ac)
	rec := httptest.NewRecorder()
	srv.handleAdminCreateInvitation(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["enrollment_token"] == "" {
		t.Fatal("missing enrollment_token")
	}
	compact := resp["invitation"]
	if compact == "" {
		t.Fatal("missing invitation compact payload")
	}
	if compact == resp["enrollment_token"] {
		t.Fatal("invitation must not equal raw enrollment token")
	}

	invitation.AllowHTTPForTest = true
	defer func() { invitation.AllowHTTPForTest = false }()
	inv, err := invitation.Parse(compact)
	if err != nil {
		t.Fatal(err)
	}
	if inv.EnrollmentToken != resp["enrollment_token"] {
		t.Fatal("enrollment token mismatch inside compact invitation")
	}
	if inv.CoordinatorFingerprint != resp["coordinator_fingerprint"] {
		t.Fatal("fingerprint mismatch")
	}
	if inv.ServerURL != "https://coordinator.test" {
		t.Fatalf("server_url %q", inv.ServerURL)
	}
}

func TestAdminListInvitationsOmitsSecrets(t *testing.T) {
	srv, _ := testServerDB(t, "")
	adminSetup(t, srv)
	ac := adminLogin(t, srv)

	createBody, _ := json.Marshal(createInvitationRequest{MaxUses: 1, ExpiresIn: 3600})
	createReq := adminCookieRequest(http.MethodPost, "/v1/admin/invitations", createBody, ac)
	createRec := httptest.NewRecorder()
	srv.handleAdminCreateInvitation(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create %d", createRec.Code)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/admin/invitations", nil)
	listReq.AddCookie(&http.Cookie{Name: adminCookieName, Value: ac.session})
	listRec := httptest.NewRecorder()
	srv.handleAdminListInvitations(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list %d", listRec.Code)
	}
	raw := listRec.Body.String()
	if strings.Contains(raw, "enrollment_token") || strings.Contains(raw, "invitation") {
		t.Fatalf("list response leaked secret fields: %s", raw)
	}
}
