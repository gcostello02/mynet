package dashboard

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// Handler serves the embedded admin SPA without intercepting API routes.
type Handler struct {
	assets fs.FS
}

func NewHandler(assets fs.FS) *Handler {
	return &Handler{assets: assets}
}

func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /assets/{path...}", h.serveAsset)
	mux.HandleFunc("GET /{$}", h.serveSPA)
	mux.HandleFunc("GET /login", h.serveSPA)
	mux.HandleFunc("GET /devices", h.serveSPA)
	mux.HandleFunc("GET /invitations", h.serveSPA)
	mux.HandleFunc("GET /settings", h.serveSPA)
}

func (h *Handler) serveSPA(w http.ResponseWriter, r *http.Request) {
	h.setSecurityHeaders(w)
	data, err := fs.ReadFile(h.assets, "index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) serveAsset(w http.ResponseWriter, r *http.Request) {
	h.setSecurityHeaders(w)
	rel := strings.TrimPrefix(r.URL.Path, "/assets/")
	rel = path.Clean(rel)
	if rel == "." || strings.Contains(rel, "..") {
		http.NotFound(w, r)
		return
	}
	fpath := path.Join("assets", rel)
	data, err := fs.ReadFile(h.assets, fpath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if ct := contentTypeFor(fpath); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self'; font-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; object-src 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
}

func contentTypeFor(name string) string {
	switch {
	case strings.HasSuffix(name, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(name, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(name, ".png"):
		return "image/png"
	case strings.HasSuffix(name, ".woff2"):
		return "font/woff2"
	case strings.HasSuffix(name, ".ico"):
		return "image/x-icon"
	default:
		return ""
	}
}
