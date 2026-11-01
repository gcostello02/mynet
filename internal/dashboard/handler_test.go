package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestHandlerDoesNotServeAPIPaths(t *testing.T) {
	fs := fstest.MapFS{
		"index.html":    {Data: []byte("<html>ok</html>")},
		"assets/app.js": {Data: []byte("console.log('x')")},
	}
	h := NewHandler(fs)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	})
	h.Mount(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("healthz: %d %q", rec.Code, rec.Body.String())
	}

	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/devices", nil))
	if rec2.Code != http.StatusOK || !strings.Contains(rec2.Body.String(), "ok</html>") {
		t.Fatalf("spa: %d %q", rec2.Code, rec2.Body.String())
	}
	if rec2.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("expected CSP header")
	}

	rec3 := httptest.NewRecorder()
	mux.ServeHTTP(rec3, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))
	if rec3.Code != http.StatusOK {
		t.Fatalf("asset: %d", rec3.Code)
	}
}
