package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func bootstrapTokenForTest(srv *Server) string {
	srv.bootstrap.mu.Lock()
	tok := srv.bootstrap.token
	srv.bootstrap.mu.Unlock()
	return tok
}

func TestSetupStatusUninitialized(t *testing.T) {
	srv, _ := testServerDB(t, "")
	req := httptest.NewRequest(http.MethodGet, "/v1/setup/status", nil)
	rec := httptest.NewRecorder()
	srv.handleSetupStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var out map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if out["initialized"] != false {
		t.Fatalf("expected uninitialized")
	}
	if out["bootstrap_active"] != true {
		t.Fatalf("expected active bootstrap")
	}
}

func TestAdminSetupRemoteRequiresBootstrap(t *testing.T) {
	srv, _ := testServerDB(t, "")
	body, _ := json.Marshal(adminSetupRequest{
		Username:           "adminuser",
		Password:           "longpassword12",
		NetworkDisplayName: "net",
		PublicServerURL:    "https://coordinator.test",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/setup", bytes.NewReader(body))
	req.RemoteAddr = "203.0.113.1:1234"
	rec := httptest.NewRecorder()
	srv.handleAdminSetup(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestAdminSetupRemoteWithBootstrap(t *testing.T) {
	srv, _ := testServerDB(t, "")
	tok := bootstrapTokenForTest(srv)
	if tok == "" {
		t.Fatal("missing bootstrap token")
	}
	body, _ := json.Marshal(adminSetupRequest{
		Username:           "adminuser",
		Password:           "longpassword12",
		NetworkDisplayName: "net",
		PublicServerURL:    "https://coordinator.test",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/setup", bytes.NewReader(body))
	req.RemoteAddr = "203.0.113.1:1234"
	req.Header.Set(bootstrapTokenHeader, tok)
	rec := httptest.NewRecorder()
	srv.handleAdminSetup(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("setup %d %s", rec.Code, rec.Body.String())
	}
	// Replay must fail.
	rec2 := httptest.NewRecorder()
	srv.handleAdminSetup(rec2, req)
	if rec2.Code != http.StatusForbidden {
		t.Fatalf("replay expected 403, got %d", rec2.Code)
	}
}

func TestBootstrapLoopbackEndpoint(t *testing.T) {
	srv, _ := testServerDB(t, "")
	req := httptest.NewRequest(http.MethodGet, "/v1/setup/bootstrap", nil)
	req.RemoteAddr = "127.0.0.1:1"
	rec := httptest.NewRecorder()
	srv.handleSetupBootstrap(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("bootstrap %d", rec.Code)
	}
	var out map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if out["bootstrap_token"] == "" {
		t.Fatal("expected token")
	}
}

func TestAdminSetupConcurrent(t *testing.T) {
	srv, _ := testServerDB(t, "")
	tok := bootstrapTokenForTest(srv)
	var wg sync.WaitGroup
	var wins int
	var mu sync.Mutex
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body, _ := json.Marshal(adminSetupRequest{
				Username:           "adminuser",
				Password:           "longpassword12",
				NetworkDisplayName: "net",
				PublicServerURL:    "https://x.test",
			})
			req := httptest.NewRequest(http.MethodPost, "/v1/admin/setup", bytes.NewReader(body))
			req.RemoteAddr = "203.0.113.1:1234"
			req.Header.Set(bootstrapTokenHeader, tok)
			rec := httptest.NewRecorder()
			srv.handleAdminSetup(rec, req)
			if rec.Code == http.StatusCreated {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("expected exactly one successful setup, got %d", wins)
	}
}

func TestSetupStatusAfterInit(t *testing.T) {
	srv, _ := testServerDB(t, "")
	_ = adminSetup(t, srv)
	req := httptest.NewRequest(http.MethodGet, "/v1/setup/status", nil)
	rec := httptest.NewRecorder()
	srv.handleSetupStatus(rec, req)
	var out map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if out["initialized"] != true {
		t.Fatalf("expected initialized")
	}
}
