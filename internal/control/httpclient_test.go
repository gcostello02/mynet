package control

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewSecureAPIClientRequiresHTTPS(t *testing.T) {
	_, err := NewSecureAPIClient("http://example.com")
	if err == nil {
		t.Fatal("expected https required")
	}
}

func TestNewSecureAPIClientBlocksCrossHostRedirect(t *testing.T) {
	AllowInsecureHTTPForTest = true
	defer func() { AllowInsecureHTTPForTest = false }()

	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"network_id":"default"}`))
	}))
	defer good.Close()
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer evil.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, evil.URL+r.URL.Path, http.StatusFound)
	}))
	defer redirector.Close()

	c, err := NewSecureAPIClient(redirector.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.NetworkPublic()
	if err == nil {
		t.Fatal("expected redirect blocked")
	}
}
