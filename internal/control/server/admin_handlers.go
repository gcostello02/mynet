package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gcostello02/mynet/internal/invitation"
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
		"network_id":              s.store.NetworkID(),
		"coordinator_public_key":  meta.CoordinatorPublicKeyB64,
		"coordinator_fingerprint": meta.CoordinatorFingerprint,
		"public_server_url":       meta.PublicServerURL,
		"network_display_name":    meta.DisplayName,
	})
}

func (s *Server) handleAdminSetup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	if !s.allowSetupRequest(r) {
		httpError(w, http.StatusForbidden, "setup not allowed from this client")
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
	if s.store.sqlDB == nil {
		httpError(w, http.StatusBadRequest, "persistent data path required for setup")
		return
	}

	s.setupMu.Lock()
	defer s.setupMu.Unlock()

	meta := s.store.Meta().Snapshot()
	if meta.SetupComplete || s.store.AdminCount() > 0 {
		httpError(w, http.StatusForbidden, "setup already completed")
		return
	}
	if deviceCount(s.store) > 0 {
		httpError(w, http.StatusForbidden, "existing enrolled devices; use V0.3 legacy mode without admin setup")
		return
	}

	id, err := GenerateCoordinatorIdentity()
	if err != nil {
		httpError(w, http.StatusInternalServerError, "identity generation failed")
		return
	}
	passHash, err := hashAdminPassword(req.Password)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "password hash failed")
		return
	}
	admin, err := firstAdminSetup(s.store.sqlDB, s.store.meta, req.Username, passHash, id, req.NetworkDisplayName, req.PublicServerURL, s.store.dbPath)
	if err != nil {
		if errors.Is(err, errSetupAlreadyDone) {
			httpError(w, http.StatusForbidden, "setup already completed")
			return
		}
		if err.Error() == "existing enrolled devices" {
			httpError(w, http.StatusForbidden, "existing enrolled devices; use V0.3 legacy mode without admin setup")
			return
		}
		httpError(w, http.StatusConflict, "admin create failed")
		return
	}
	s.consumeBootstrapCapability()
	s.store.admins[admin.ID] = admin
	s.store.adminByUser[admin.Username] = admin.ID
	s.coordinatorIdentity = id
	writeJSON(w, http.StatusCreated, map[string]string{
		"coordinator_fingerprint": id.Fingerprint,
		"admin_id":                admin.ID,
	})
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

type adminPasswordChangeRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (s *Server) handleAdminPasswordChange(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.adminRequire(w, r, true)
	if !ok {
		return
	}
	var req adminPasswordChangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(req.NewPassword) < 12 {
		httpError(w, http.StatusBadRequest, "password requirements not met")
		return
	}
	admin := s.store.admins[sess.AdminID]
	if admin == nil {
		httpError(w, http.StatusUnauthorized, "admin authentication required")
		return
	}
	if !verifyAdminPassword(req.CurrentPassword, admin.PasswordHash) {
		httpError(w, http.StatusForbidden, "invalid current password")
		return
	}
	newHash, err := hashAdminPassword(req.NewPassword)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "password hash failed")
		return
	}
	if s.store.sqlDB != nil {
		if err := updateAdminPasswordHash(s.store.sqlDB, admin.ID, newHash); err != nil {
			httpError(w, http.StatusInternalServerError, "password update failed")
			return
		}
	}
	admin.PasswordHash = newHash
	s.store.adminSessions.deleteAll()
	s.clearAdminSessionCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) clearAdminSessionCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: adminCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.cfg.SecureCookies, SameSite: http.SameSiteStrictMode})
	http.SetCookie(w, &http.Cookie{Name: adminCSRFCookie, Value: "", Path: "/", MaxAge: -1, Secure: s.cfg.SecureCookies, SameSite: http.SameSiteStrictMode})
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
			"device_id":        d.DeviceID,
			"name":             d.Name,
			"vpn_ipv4":         d.VPNIPv4,
			"revoked":          d.Revoked,
			"signaling_online": s.hub.IsOnline(d.DeviceID),
			"enrolled_at":      d.Enrolled.Format(time.RFC3339),
			"auth_epoch":       d.AuthEpoch,
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
	compact, err := invitation.Compact(
		meta.PublicServerURL,
		s.store.NetworkID(),
		meta.CoordinatorFingerprint,
		meta.CoordinatorPublicKeyB64,
		token,
	)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "invitation encode failed")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"id":                      id,
		"enrollment_token":        token,
		"invitation":              compact,
		"coordinator_fingerprint": meta.CoordinatorFingerprint,
		"public_server_url":       meta.PublicServerURL,
		"network_id":              s.store.NetworkID(),
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
