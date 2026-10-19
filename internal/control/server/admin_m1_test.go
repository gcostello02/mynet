package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/grantcostello/mynet/internal/control"
	"github.com/grantcostello/mynet/internal/identity"
)

func testServerDB(t *testing.T, enrollSecret string) (*Server, string) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "coordinator.db")
	srv, err := New(Config{
		ListenAddr:       ":0",
		ServerSecret:     bytes.Repeat([]byte("s"), 32),
		EnrollmentSecret: enrollSecret,
		NetworkID:        "testnet",
		DataPath:         dbPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	return srv, dbPath
}

func adminSetup(t *testing.T, srv *Server) string {
	body, _ := json.Marshal(adminSetupRequest{
		Username:           "adminuser",
		Password:           "longpassword12",
		NetworkDisplayName: "test",
		PublicServerURL:    "https://coordinator.test",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/setup", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	srv.handleAdminSetup(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("setup %d %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	return resp["coordinator_fingerprint"]
}

type adminCookies struct {
	session string
	csrf    string
}

func adminLogin(t *testing.T, srv *Server) adminCookies {
	body, _ := json.Marshal(adminLoginRequest{Username: "adminuser", Password: "longpassword12"})
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

func TestAdminSetupUnauthorizedRemote(t *testing.T) {
	srv, _ := testServerDB(t, "")
	body, _ := json.Marshal(adminSetupRequest{Username: "adminuser", Password: "longpassword12"})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/setup", bytes.NewReader(body))
	req.RemoteAddr = "203.0.113.1:1234"
	rec := httptest.NewRecorder()
	srv.handleAdminSetup(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestAdminFlowInvitationEnrollRevoke(t *testing.T) {
	srv, _ := testServerDB(t, "")
	fp := adminSetup(t, srv)
	ac := adminLogin(t, srv)

	invBody, _ := json.Marshal(createInvitationRequest{Label: "laptop", MaxUses: 1, ExpiresIn: 3600})
	invReq := httptest.NewRequest(http.MethodPost, "/v1/admin/invitations", bytes.NewReader(invBody))
	invReq.AddCookie(&http.Cookie{Name: adminCookieName, Value: ac.session})
	invReq.AddCookie(&http.Cookie{Name: adminCSRFCookie, Value: ac.csrf})
	invReq.Header.Set(adminCSRFHeader, ac.csrf)
	invRec := httptest.NewRecorder()
	srv.handleAdminCreateInvitation(invRec, invReq)
	if invRec.Code != http.StatusCreated {
		t.Fatalf("invite %d %s", invRec.Code, invRec.Body.String())
	}
	var invResp map[string]string
	_ = json.NewDecoder(invRec.Body).Decode(&invResp)
	token := invResp["enrollment_token"]

	id, err := identity.Generate("invited")
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Now().UTC().Unix()
	msg := identity.CanonicalEnrollMessage(id.DeviceID, id.Name, ts)
	sig, _ := id.Sign(msg)
	enrollBody, _ := json.Marshal(control.EnrollRequest{
		DeviceID:               id.DeviceID,
		Name:                   id.Name,
		PublicKey:              id.PublicKey,
		Timestamp:              ts,
		Signature:              base64.StdEncoding.EncodeToString(sig),
		EnrollmentToken:        token,
		CoordinatorFingerprint: fp,
	})
	enrollReq := httptest.NewRequest(http.MethodPost, "/v1/enroll", bytes.NewReader(enrollBody))
	enrollRec := httptest.NewRecorder()
	srv.handleEnroll(enrollRec, enrollReq)
	if enrollRec.Code != http.StatusCreated {
		t.Fatalf("enroll %d %s", enrollRec.Code, enrollRec.Body.String())
	}
	devTok := authDevice(t, srv, id)

	revReq := httptest.NewRequest(http.MethodPost, "/v1/admin/devices/"+id.DeviceID+"/revoke", nil)
	revReq.SetPathValue("id", id.DeviceID)
	revReq.AddCookie(&http.Cookie{Name: adminCookieName, Value: ac.session})
	revReq.AddCookie(&http.Cookie{Name: adminCSRFCookie, Value: ac.csrf})
	revReq.Header.Set(adminCSRFHeader, ac.csrf)
	revRec := httptest.NewRecorder()
	srv.handleAdminRevokeDevice(revRec, revReq)
	if revRec.Code != http.StatusNoContent {
		t.Fatalf("revoke %d", revRec.Code)
	}

	peersReq := httptest.NewRequest(http.MethodGet, "/v1/peers", nil)
	peersReq.Header.Set("Authorization", "Bearer "+devTok)
	peersRec := httptest.NewRecorder()
	srv.withAuth(srv.handlePeers)(peersRec, peersReq)
	if peersRec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token should fail peers, got %d", peersRec.Code)
	}
}

func TestLegacyEnrollmentWithDB(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v03.db")
	db, err := openCoordinatorDB(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO devices (device_id, name, public_key, network_id, revoked, enrolled_at, wg_public_key, vpn_ipv4)
		VALUES ('seed', 'seed', 'cHVibGlj', 'testnet', 0, ?, '', '')`, time.Now().UTC().Format(time.RFC3339Nano))
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	srv, err := New(Config{
		ListenAddr:       ":0",
		ServerSecret:     bytes.Repeat([]byte("s"), 32),
		EnrollmentSecret: "test-enroll-secret",
		NetworkID:        "testnet",
		DataPath:         path,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, tok := enrollDevice(t, srv, "legacy-node")
	req := httptest.NewRequest(http.MethodGet, "/v1/peers", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	srv.withAuth(srv.handlePeers)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("legacy peers %d", rec.Code)
	}
}

func TestInvitationConcurrentRedeem(t *testing.T) {
	srv, _ := testServerDB(t, "")
	_ = adminSetup(t, srv)
	fp := srv.store.Meta().Snapshot().CoordinatorFingerprint
	id, token, err := srv.store.CreateInvitation("race", time.Hour, 1, "admin")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var wins int
	var mu sync.Mutex
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dev, err := identity.Generate("race")
			if err != nil {
				return
			}
			ts := time.Now().UTC().Unix()
			msg := identity.CanonicalEnrollMessage(dev.DeviceID, dev.Name, ts)
			sig, _ := dev.Sign(msg)
			body, _ := json.Marshal(control.EnrollRequest{
				DeviceID:               dev.DeviceID,
				Name:                   dev.Name,
				PublicKey:              dev.PublicKey,
				Timestamp:              ts,
				Signature:              base64.StdEncoding.EncodeToString(sig),
				EnrollmentToken:        token,
				CoordinatorFingerprint: fp,
			})
			req := httptest.NewRequest(http.MethodPost, "/v1/enroll", bytes.NewReader(body))
			rec := httptest.NewRecorder()
			srv.handleEnroll(rec, req)
			if rec.Code == http.StatusCreated {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("expected 1 successful enroll, got %d (invitation %s)", wins, id)
	}
}

func TestRevokedIPReserved(t *testing.T) {
	srv := testServer(t)
	id, _ := enrollDevice(t, srv, "ip-holder")
	ip, err := srv.store.AllocateVPNIPv4(id.DeviceID, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", srv.vpnPool())
	if err != nil {
		t.Fatal(err)
	}
	srv.store.RevokeDevice(id.DeviceID)
	id2, _ := enrollDevice(t, srv, "ip-next")
	ip2, err := srv.store.AllocateVPNIPv4(id2.DeviceID, "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210", srv.vpnPool())
	if err != nil {
		t.Fatal(err)
	}
	if ip == ip2 {
		t.Fatalf("revoked device IP %s was reallocated", ip)
	}
}
