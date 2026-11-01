package server

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/gcostello02/mynet/internal/control"
)


// adminRequire validates admin session; cookie-authenticated mutations need same-origin + CSRF.
func (s *Server) adminRequire(w http.ResponseWriter, r *http.Request, mutating bool) (*AdminSession, bool) {
	sess, ok := s.adminSessionFromRequest(r)
	if !ok {
		httpError(w, http.StatusUnauthorized, "admin authentication required")
		return nil, false
	}
	if mutating && bearerToken(r) == "" {
		if !adminSameOrigin(r) {
			httpError(w, http.StatusForbidden, "forbidden")
			return nil, false
		}
		if !s.adminCSRFValid(r, sess) {
			httpError(w, http.StatusForbidden, "forbidden")
			return nil, false
		}
	}
	return sess, true
}

func adminSameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		if r.Header.Get("Referer") == "" {
			// Non-browser clients (CLI, tests) do not send Origin/Referer.
			return true
		}
		u, err := url.Parse(r.Header.Get("Referer"))
		if err != nil {
			return false
		}
		return hostsEqual(u.Host, r.Host)
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return hostsEqual(u.Host, r.Host)
}

func hostsEqual(a, b string) bool {
	return normalizeHost(a) == normalizeHost(b)
}

func normalizeHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		return host
	}
	return h
}

func (s *Server) adminNoStoreMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/admin/") || strings.HasPrefix(r.URL.Path, "/v1/setup/") {
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) invalidateAdminSessionCookie(r *http.Request) {
	c, err := r.Cookie(adminCookieName)
	if err != nil || c.Value == "" {
		return
	}
	s.store.adminSessions.delete(c.Value)
}

func adminLoginIncludesSessionToken(r *http.Request) bool {
	return r.Header.Get(control.AdminClientCLIHeader) == control.AdminClientCLIValue
}
