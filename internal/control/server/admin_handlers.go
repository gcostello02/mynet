package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gcostello02/mynet/internal/version"
)

type adminSetupRequest struct {
	Username           string `json:"username"`
	Password           string `json:"password"`
	NetworkDisplayName string `json:"network_display_name"`
	PublicServerURL    string `json:"public_server_url"`
}

type adminLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleNetworkPublic(w http.ResponseWriter, r *http.Request) {
	meta := s.store.Meta().Snapshot()
	if !meta.SetupComplete {
		httpError(w, http.StatusNotFound, "network not initialized")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"network_id":               s.store.NetworkID(),
		"coordinator_public_key":   meta.CoordinatorPublicKeyB64,
		"coordinator_fingerprint":  meta.CoordinatorFingerprint,
		"public_server_url":        meta.PublicServerURL,
		"network_display_name":     meta.DisplayName,
	})
}

func (s *Server) handleAdminSetup(w http.ResponseWriter, r *http.Request) {
	if !s.allowSetupRequest(r) {
		httpError(w, http.StatusForbidden, "setup not allowed from this client")
		return
	}
	meta := s.store.Meta().Snapshot()
	if meta.SetupComplete || s.store.AdminCount() > 0 {
		httpError(w, http.StatusForbidden, "setup already completed")
		return
	}
	if deviceCount(s.store) > 0 {
		httpError(w, http.StatusForbidden, "existing enrolled devices; use V0.3 legacy mode without admin setup")
		return
	}
	var req adminSetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(req.Username) < 3 || len(req.Password) < 12 {
		httpError(w, http.StatusBadRequest, "username and password requirements not met")
		return
	}
	id, err := GenerateCoordinatorIdentity()
	if err != nil {
		httpError(w, http.StatusInternalServerError, "identity generation failed")
		return
	}
	hash, err := hashAdminPassword(req.Password)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "password hash failed")
		return
	}
	if s.store.sqlDB == nil {
		httpError(w, http.StatusBadRequest, "persistent data path required for setup")
		return
	}
	admin, err := insertAdmin(s.store.sqlDB, req.Username, hash)
	if err != nil {
		httpError(w, http.StatusConflict, "admin create failed")
		return
	}
	if err := completeSetup(s.store.sqlDB, s.store.meta, id, req.NetworkDisplayName, req.PublicServerURL, s.store.dbPath); err != nil {
		httpError(w, http.StatusInternalServerError, "setup failed")
		return
	}
	s.store.admins[admin.ID] = admin
	s.store.adminByUser[admin.Username] = admin.ID
	s.coordinatorIdentity = id
	writeJSON(w, http.StatusCreated, map[string]string{
		"coordinator_fingerprint": id.Fingerprint,
		"admin_id":                admin.ID,
	})
}

func (s *Server) allowSetupRequest(r *http.Request) bool {
	if isLoopback(r) {
		return true
	}
	if s.cfg.SetupToken == "" {
		return false
	}
	got := r.Header.Get(setupTokenHeader)
	return got != "" && constantTimeStringEqual(got, s.cfg.SetupToken)
}

func (s *Server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	var req adminLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid json")
		return
	}
	adminID, ok := s.store.adminByUser[req.Username]
	if !ok {
		httpError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	admin := s.store.admins[adminID]
	if admin == nil || !verifyAdminPassword(req.Password, admin.PasswordHash) {
		httpError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	s.invalidateAdminSessionCookie(r)
	sess, err := s.store.adminSessions.create(admin.ID)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "session create failed")
		return
	}
	s.setAdminSessionCookies(w, sess)
	resp := map[string]string{
		"admin_id":   admin.ID,
		"expires_at": sess.ExpiresAt.Format(time.RFC3339),
	}
	if adminLoginIncludesSessionToken(r) {
		resp["session_token"] = sess.ID
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.adminRequire(w, r, true)
	if !ok {
		return
	}
	s.store.adminSessions.delete(sess.ID)
	http.SetCookie(w, &http.Cookie{Name: adminCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.cfg.SecureCookies, SameSite: http.SameSiteStrictMode})
	http.SetCookie(w, &http.Cookie{Name: adminCSRFCookie, Value: "", Path: "/", MaxAge: -1, SameSite: http.SameSiteStrictMode})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminStatus(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.adminRequire(w, r, false); !ok {
		return
	}
	meta := s.store.Meta().Snapshot()
	devs := s.store.ListAllDevices()
	revoked := 0
	online := 0
	for _, d := range devs {
		if d.Revoked {
			revoked++
			continue
		}
		if s.hub.IsOnline(d.DeviceID) {
			online++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"setup_complete":          meta.SetupComplete,
		"device_count":            len(devs),
		"active_device_count":     len(devs) - revoked,
		"revoked_device_count":    revoked,
		"signaling_online_count":  online,
		"legacy_enrollment":       meta.LegacyEnrollmentEnabled,
		"coordinator_fingerprint": meta.CoordinatorFingerprint,
		"server_version":          version.Version,
	})
}

func (s *Server) handleAdminListDevices(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.adminRequire(w, r, false); !ok {
		return
	}
	devs := s.store.ListAllDevices()
	out := make([]map[string]any, 0, len(devs))
	for _, d := range devs {
		out = append(out, map[string]any{
			"device_id":         d.DeviceID,
			"name":              d.Name,
			"vpn_ipv4":          d.VPNIPv4,
			"revoked":           d.Revoked,
			"signaling_online":  s.hub.IsOnline(d.DeviceID),
			"enrolled_at":       d.Enrolled.Format(time.RFC3339),
			"auth_epoch":        d.AuthEpoch,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleAdminPatchDevice(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.adminRequire(w, r, true)
	if !ok {
		return
	}
	_ = admin
	deviceID := r.PathValue("id")
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Name == "" {
		httpError(w, http.StatusBadRequest, "name required")
		return
	}
	if err := s.store.RenameDevice(deviceID, body.Name); err != nil {
		if err == errNameTaken {
			httpError(w, http.StatusConflict, err.Error())
			return
		}
		httpError(w, http.StatusNotFound, "device not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminRevokeDevice(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.adminRequire(w, r, true); !ok {
		return
	}
	deviceID := r.PathValue("id")
	if !s.store.RevokeDevice(deviceID) {
		httpError(w, http.StatusNotFound, "device not found")
		return
	}
	s.hub.DisconnectDevice(deviceID)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminListInvitations(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.adminRequire(w, r, false); !ok {
		return
	}
	invites := s.store.ListInvitations()
	out := make([]map[string]any, 0, len(invites))
	for _, inv := range invites {
		out = append(out, map[string]any{
			"id":         inv.ID,
			"label":      inv.Label,
			"expires_at": inv.ExpiresAt.Format(time.RFC3339),
			"max_uses":   inv.MaxUses,
			"uses":       inv.Uses,
			"revoked":    !inv.RevokedAt.IsZero(),
			"created_at": inv.CreatedAt.Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

type createInvitationRequest struct {
	Label     string `json:"label"`
	ExpiresIn int    `json:"expires_in_seconds"`
	MaxUses   int    `json:"max_uses"`
}

func (s *Server) handleAdminCreateInvitation(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.adminRequire(w, r, true)
	if !ok {
		return
	}
	var req createInvitationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid json")
		return
	}
	expires := time.Duration(req.ExpiresIn) * time.Second
	if expires == 0 {
		expires = 24 * time.Hour
	}
	id, token, err := s.store.CreateInvitation(req.Label, expires, req.MaxUses, admin.ID)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "invitation create failed")
		return
	}
	meta := s.store.Meta().Snapshot()
	writeJSON(w, http.StatusCreated, map[string]string{
		"id":                      id,
		"enrollment_token":        token,
		"coordinator_fingerprint": meta.CoordinatorFingerprint,
		"public_server_url":       meta.PublicServerURL,
	})
}

func (s *Server) handleAdminRevokeInvitation(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.adminRequire(w, r, true); !ok {
		return
	}
	id := r.PathValue("id")
	if !s.store.RevokeInvitation(id) {
		httpError(w, http.StatusNotFound, "invitation not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) setAdminSessionCookies(w http.ResponseWriter, sess *AdminSession) {
	http.SetCookie(w, &http.Cookie{
		Name:     adminCookieName,
		Value:    sess.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.SecureCookies,
		SameSite: http.SameSiteStrictMode,
		Expires:  sess.ExpiresAt,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     adminCSRFCookie,
		Value:    sess.CSRFToken,
		Path:     "/",
		Secure:   s.cfg.SecureCookies,
		SameSite: http.SameSiteStrictMode,
		Expires:  sess.ExpiresAt,
	})
}

func (s *Server) adminSessionFromRequest(r *http.Request) (*AdminSession, bool) {
	if auth := bearerToken(r); auth != "" {
		sess, ok := s.store.adminSessions.get(auth)
		return sess, ok
	}
	c, err := r.Cookie(adminCookieName)
	if err != nil || c.Value == "" {
		return nil, false
	}
	sess, ok := s.store.adminSessions.get(c.Value)
	return sess, ok
}

func (s *Server) adminCSRFValid(r *http.Request, sess *AdminSession) bool {
	if bearerToken(r) != "" {
		return true
	}
	header := r.Header.Get(adminCSRFHeader)
	if header == "" || header != sess.CSRFToken {
		return false
	}
	if c, err := r.Cookie(adminCSRFCookie); err != nil || c.Value != sess.CSRFToken {
		return false
	}
	return true
}

type legacyEnrollmentRequest struct {
	Enabled bool `json:"enabled"`
}

func (s *Server) handleAdminLegacyEnrollment(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.adminRequire(w, r, true); !ok {
		return
	}
	if s.store.sqlDB == nil {
		httpError(w, http.StatusBadRequest, "persistent database required")
		return
	}
	var req legacyEnrollmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid json")
		return
	}
	val := "false"
	if req.Enabled {
		if s.store.EnrollmentSecretHash() == "" {
			httpError(w, http.StatusBadRequest, "MYNET_ENROLLMENT_SECRET not configured on coordinator")
			return
		}
		val = "true"
	}
	if err := setMetaValue(s.store.sqlDB, metaLegacyEnrollmentEnabled, val); err != nil {
		httpError(w, http.StatusInternalServerError, "update failed")
		return
	}
	s.store.meta.mu.Lock()
	s.store.meta.LegacyEnrollmentEnabled = req.Enabled
	s.store.meta.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

type controlEnrollFields struct {
	EnrollmentSecret       string
	EnrollmentToken        string
	CoordinatorFingerprint string
}
