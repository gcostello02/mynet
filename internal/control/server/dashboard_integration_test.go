package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDashboardAndAPIRoutesCoexist(t *testing.T) {
	srv := testServer(t)
	h := srv.Handler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/peers", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("api route should not be SPA, got %d", rec.Code)
	}

	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec2.Code != http.StatusOK && rec2.Code != http.StatusNotFound {
		t.Fatalf("dashboard root: %d", rec2.Code)
	}
	if rec2.Code == http.StatusOK && rec2.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("expected CSP on dashboard")
	}
}
