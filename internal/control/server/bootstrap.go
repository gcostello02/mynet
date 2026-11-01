package server

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

const (
	bootstrapTokenHeader = "X-Mynet-Bootstrap-Token"
	bootstrapTTL         = 2 * time.Hour

	metaBootstrapTokenHash   = "bootstrap_token_hash"
	metaBootstrapExpiresAt   = "bootstrap_expires_at"
)

// bootstrapSlot holds the active one-time bootstrap capability (plaintext only in memory).
type bootstrapSlot struct {
	mu      sync.Mutex
	token   string
	hash    string
	expires time.Time
}

func hashBootstrapToken(token string) string {
	sum := sha256.Sum256([]byte("mynet-bootstrap-v1\n" + token))
	return hex.EncodeToString(sum[:])
}

func (s *Server) initBootstrapIfNeeded() error {
	if s.store.sqlDB == nil {
		return nil
	}
	if s.store.meta.SetupComplete || s.store.AdminCount() > 0 {
		return nil
	}
	return s.rotateBootstrapCapability()
}

func (s *Server) rotateBootstrapCapability() error {
	token, err := newRandomToken(32)
	if err != nil {
		return err
	}
	hash := hashBootstrapToken(token)
	expires := time.Now().UTC().Add(bootstrapTTL)
	if err := setMetaValue(s.store.sqlDB, metaBootstrapTokenHash, hash); err != nil {
		return err
	}
	if err := setMetaValue(s.store.sqlDB, metaBootstrapExpiresAt, expires.Format(time.RFC3339Nano)); err != nil {
		return err
	}
	s.bootstrap.mu.Lock()
	s.bootstrap.token = token
	s.bootstrap.hash = hash
	s.bootstrap.expires = expires
	s.bootstrap.mu.Unlock()
	return nil
}

func (s *Server) bootstrapActive() (expires time.Time, ok bool) {
	if s.store.meta.SetupComplete {
		return time.Time{}, false
	}
	s.bootstrap.mu.Lock()
	defer s.bootstrap.mu.Unlock()
	if s.bootstrap.hash == "" || s.bootstrap.token == "" {
		return time.Time{}, false
	}
	if time.Now().After(s.bootstrap.expires) {
		return s.bootstrap.expires, false
	}
	return s.bootstrap.expires, true
}

func (s *Server) verifyBootstrapToken(token string) bool {
	if token == "" {
		return false
	}
	s.bootstrap.mu.Lock()
	hash := s.bootstrap.hash
	expires := s.bootstrap.expires
	s.bootstrap.mu.Unlock()
	if hash == "" {
		return false
	}
	if time.Now().After(expires) {
		return false
	}
	return constantTimeStringEqual(hashBootstrapToken(token), hash)
}

func (s *Server) consumeBootstrapCapability() {
	s.bootstrap.mu.Lock()
	s.bootstrap.token = ""
	s.bootstrap.hash = ""
	s.bootstrap.expires = time.Time{}
	s.bootstrap.mu.Unlock()
	if s.store.sqlDB != nil {
		_ = setMetaValue(s.store.sqlDB, metaBootstrapTokenHash, "")
		_ = setMetaValue(s.store.sqlDB, metaBootstrapExpiresAt, "")
	}
}

// BootstrapConsoleMessage prints operator instructions to stderr (never via structured logs).
func (s *Server) BootstrapConsoleMessage() string {
	if s.store.meta.SetupComplete {
		return ""
	}
	exp, ok := s.bootstrapActive()
	if !ok {
		return ""
	}
	s.bootstrap.mu.Lock()
	tok := s.bootstrap.token
	s.bootstrap.mu.Unlock()
	if tok == "" {
		return ""
	}
	return fmt.Sprintf(
		"MyNet coordinator is uninitialized.\n"+
			"Bootstrap capability (one-time setup, expires %s UTC):\n%s\n"+
			"Reveal this only on the coordinator host. Remote setup requires header %s.\n"+
			"Loopback clients may also call GET /v1/setup/bootstrap.\n",
		exp.UTC().Format(time.RFC3339),
		tok,
		bootstrapTokenHeader,
	)
}

// WriteBootstrapConsole writes the bootstrap notice to stderr without logging the secret.
func (s *Server) WriteBootstrapConsole() {
	msg := s.BootstrapConsoleMessage()
	if msg != "" {
		_, _ = os.Stderr.WriteString(msg)
	}
}

func (s *Server) allowSetupRequest(r *http.Request) bool {
	if isLoopback(r) {
		return true
	}
	if s.cfg.SetupToken != "" {
		got := r.Header.Get(setupTokenHeader)
		if got != "" && constantTimeStringEqual(got, s.cfg.SetupToken) {
			return true
		}
	}
	return s.verifyBootstrapToken(r.Header.Get(bootstrapTokenHeader))
}

func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	meta := s.store.Meta().Snapshot()
	if meta.SetupComplete {
		writeJSON(w, http.StatusOK, map[string]any{
			"initialized": true,
		})
		return
	}
	exp, active := s.bootstrapActive()
	out := map[string]any{
		"initialized":              false,
		"requires_bootstrap_proof": true,
		"bootstrap_active":         active,
	}
	if active {
		out["bootstrap_expires_at"] = exp.UTC().Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleSetupBootstrap(w http.ResponseWriter, r *http.Request) {
	if !isLoopback(r) {
		httpError(w, http.StatusForbidden, "forbidden")
		return
	}
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	if s.store.meta.SetupComplete {
		httpError(w, http.StatusNotFound, "network already initialized")
		return
	}
	exp, active := s.bootstrapActive()
	if !active {
		httpError(w, http.StatusGone, "bootstrap capability expired; restart the coordinator for a new capability")
		return
	}
	s.bootstrap.mu.Lock()
	tok := s.bootstrap.token
	s.bootstrap.mu.Unlock()
	if tok == "" {
		httpError(w, http.StatusGone, "bootstrap capability unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"bootstrap_token":    tok,
		"bootstrap_expires_at": exp.UTC().Format(time.RFC3339),
	})
}
