package server

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/gcostello02/mynet/internal/control"
	"github.com/gcostello02/mynet/internal/dashboard"
	"github.com/gcostello02/mynet/internal/identity"
)

const (
	maxBodyBytes  = 64 * 1024
	authTimeSkew  = 5 * time.Minute
	maxWSMsgBytes = 16 * 1024
)

// Config configures the coordination server.
type Config struct {
	ListenAddr       string
	ServerSecret     []byte
	EnrollmentSecret string
	NetworkID        string
	Logger           *slog.Logger
	ICE              ICEConfig
	VPN              VPNConfig
	DataPath         string // SQLite path for device persistence
	SetupToken       string // optional remote bootstrap token (X-Mynet-Setup-Token)
	SecureCookies    bool   // set true behind HTTPS
}

// Server is the MyNet coordination / signaling server.
type Server struct {
	cfg                 Config
	store               *Store
	hub                 *Hub
	http                *http.Server
	log                 *slog.Logger
	enrollRL            *ipLimiter
	loginRL             *ipLimiter
	coordinatorIdentity *CoordinatorIdentity
	bootstrap           bootstrapSlot
	setupMu             sync.Mutex
}

func New(cfg Config) (*Server, error) {
	if len(cfg.ServerSecret) < 32 {
		return nil, errors.New("server secret must be at least 32 bytes")
	}
	if cfg.NetworkID == "" {
		cfg.NetworkID = "default"
	}
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	store := NewStore(cfg.NetworkID, "")
	if cfg.EnrollmentSecret != "" {
		store.enrollmentSecretHash = hashEnrollmentSecret(cfg.EnrollmentSecret)
	}
	if cfg.DataPath != "" {
		if err := attachCoordinatorDB(store, cfg.DataPath); err != nil {
			return nil, fmt.Errorf("load coordinator db: %w", err)
		}
		log.Info("loaded devices from database", "path", cfg.DataPath, "count", deviceCount(store))
		// V0.3 migration: legacy enrollment only when devices already exist in DB.
		if cfg.EnrollmentSecret != "" && deviceCount(store) > 0 && !store.meta.LegacyEnrollmentEnabled {
			if err := enableLegacyEnrollmentMeta(store.sqlDB, store.meta); err != nil {
				return nil, fmt.Errorf("enable legacy enrollment: %w", err)
			}
		}
	} else if cfg.EnrollmentSecret != "" {
		store.meta.LegacyEnrollmentEnabled = true
	}
	s := &Server{
		cfg:      cfg,
		store:    store,
		hub:      NewHub(log),
		log:      log,
		enrollRL: newIPLimiter(30, 10),
		loginRL:  newIPLimiter(10, 5),
	}
	if cfg.DataPath != "" && store.meta.SetupComplete {
		id, err := loadCoordinatorIdentity(cfg.DataPath, store.meta.CoordinatorPublicKeyB64, store.meta.CoordinatorFingerprint)
		if err != nil {
			return nil, fmt.Errorf("coordinator identity: %w", err)
		}
		s.coordinatorIdentity = id
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.Handle("POST /v1/enroll", s.rateLimitMiddleware(s.enrollRL, http.HandlerFunc(s.handleEnroll)))
	mux.HandleFunc("POST /v1/auth", s.handleAuth)
	mux.HandleFunc("GET /v1/network/public", s.handleNetworkPublic)
	mux.Handle("GET /v1/setup/status", s.rateLimitMiddleware(s.loginRL, http.HandlerFunc(s.handleSetupStatus)))
	mux.Handle("GET /v1/setup/bootstrap", s.rateLimitMiddleware(s.loginRL, http.HandlerFunc(s.handleSetupBootstrap)))
	mux.Handle("POST /v1/network/proof", s.rateLimitMiddleware(s.enrollRL, http.HandlerFunc(s.handleNetworkProof)))
	mux.Handle("POST /v1/admin/setup", s.rateLimitMiddleware(s.loginRL, http.HandlerFunc(s.handleAdminSetup)))
	mux.Handle("POST /v1/admin/login", s.rateLimitMiddleware(s.loginRL, http.HandlerFunc(s.handleAdminLogin)))
	mux.Handle("POST /v1/admin/password", s.rateLimitMiddleware(s.loginRL, http.HandlerFunc(s.handleAdminPasswordChange)))
	mux.HandleFunc("POST /v1/admin/logout", s.handleAdminLogout)
	mux.HandleFunc("GET /v1/admin/status", s.handleAdminStatus)
	mux.HandleFunc("GET /v1/admin/devices", s.handleAdminListDevices)
	mux.HandleFunc("PATCH /v1/admin/devices/{id}", s.handleAdminPatchDevice)
	mux.HandleFunc("POST /v1/admin/devices/{id}/revoke", s.handleAdminRevokeDevice)
	mux.HandleFunc("GET /v1/admin/invitations", s.handleAdminListInvitations)
	mux.HandleFunc("POST /v1/admin/invitations", s.handleAdminCreateInvitation)
	mux.HandleFunc("DELETE /v1/admin/invitations/{id}", s.handleAdminRevokeInvitation)
	mux.HandleFunc("PUT /v1/admin/network/legacy-enrollment", s.handleAdminLegacyEnrollment)
	mux.HandleFunc("GET /v1/peers", s.withAuth(s.handlePeers))
	mux.HandleFunc("GET /v1/ice-servers", s.withAuth(s.handleICEServersAuth))
	mux.HandleFunc("POST /v1/sessions", s.withAuth(s.handleCreateSession))
	mux.HandleFunc("GET /v1/ws", s.handleWebSocket)
	mux.HandleFunc("POST /v1/vpn/register", s.withAuth(s.handleVPNRegister))
	mux.HandleFunc("GET /v1/vpn/config", s.withAuth(s.handleVPNConfig))

	if assets, err := dashboard.FS(); err == nil {
		dashboard.NewHandler(assets).Mount(mux)
	}

	s.http = &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           s.limitBody(s.adminNoStoreMiddleware(mux)),
		ReadHeaderTimeout: 10 * time.Second,
	}
	if err := s.initBootstrapIfNeeded(); err != nil {
		return nil, fmt.Errorf("bootstrap: %w", err)
	}
	return s, nil
}

func (s *Server) ListenAndServe() error {
	s.log.Info("mynet-server listening", "addr", s.cfg.ListenAddr)
	return s.http.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

// Handler exposes the HTTP handler (for tests and custom listeners).
func (s *Server) Handler() http.Handler {
	return s.http.Handler
}

func (s *Server) Store() *Store { return s.store }
func (s *Server) Hub() *Hub     { return s.hub }

func (s *Server) limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleEnroll(w http.ResponseWriter, r *http.Request) {
	var req control.EnrollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := validateTimestamp(req.Timestamp, authTimeSkew); err != nil {
		httpError(w, http.StatusBadRequest, "invalid timestamp")
		return
	}
	enrollFields := controlEnrollFields{
		EnrollmentSecret:       req.EnrollmentSecret,
		EnrollmentToken:        req.EnrollmentToken,
		CoordinatorFingerprint: req.CoordinatorFingerprint,
	}
	if err := s.validateEnrollmentGate(enrollFields); err != nil {
		code := http.StatusUnauthorized
		if err == errInvitationExpired || err == errInvitationExhausted {
			code = http.StatusGone
		}
		httpError(w, code, "enrollment denied")
		return
	}
	invitationToken := req.EnrollmentToken
	pub, err := base64.StdEncoding.DecodeString(req.PublicKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		httpError(w, http.StatusBadRequest, "invalid public key")
		return
	}
	if identity.DeviceIDFromPublicKey(pub) != req.DeviceID {
		httpError(w, http.StatusBadRequest, "device_id mismatch")
		return
	}
	sig, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil {
		httpError(w, http.StatusBadRequest, "invalid signature encoding")
		return
	}
	msg := identity.CanonicalEnrollMessage(req.DeviceID, req.Name, req.Timestamp)
	if !ed25519.Verify(pub, msg, sig) {
		httpError(w, http.StatusUnauthorized, "signature verification failed")
		return
	}
	rec := &DeviceRecord{
		DeviceID:  req.DeviceID,
		Name:      req.Name,
		PublicKey: req.PublicKey,
		NetworkID: s.store.NetworkID(),
		Enrolled:  time.Now().UTC(),
	}
	var enrollErr error
	if invitationToken != "" {
		enrollErr = s.store.EnrollDeviceWithInvitationAtomic(invitationToken, rec)
	} else {
		enrollErr = s.store.EnrollDevice(rec)
	}
	if enrollErr != nil {
		if enrollErr == errNameTaken || enrollErr == errDeviceExists || enrollErr == errRevoked {
			code := http.StatusConflict
			if enrollErr == errRevoked {
				code = http.StatusForbidden
			}
			if enrollErr == errInvitationExpired || enrollErr == errInvitationExhausted || enrollErr == errInvitationInvalid {
				code = http.StatusGone
			}
			httpError(w, code, enrollErr.Error())
			return
		}
		if enrollErr == errInvitationExpired || enrollErr == errInvitationExhausted || enrollErr == errInvitationInvalid {
			httpError(w, http.StatusGone, "enrollment denied")
			return
		}
		httpError(w, http.StatusBadRequest, enrollErr.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"device_id":  req.DeviceID,
		"name":       req.Name,
		"network_id": s.store.NetworkID(),
	})
}

func (s *Server) handleAuth(w http.ResponseWriter, r *http.Request) {
	var req control.AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := validateTimestamp(req.Timestamp, authTimeSkew); err != nil {
		httpError(w, http.StatusBadRequest, "invalid timestamp")
		return
	}
	dev, ok := s.store.GetDevice(req.DeviceID)
	if !ok {
		httpError(w, http.StatusUnauthorized, "unknown device")
		return
	}
	pub, err := base64.StdEncoding.DecodeString(dev.PublicKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		httpError(w, http.StatusInternalServerError, "stored public key invalid")
		return
	}
	sig, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil {
		httpError(w, http.StatusBadRequest, "invalid signature encoding")
		return
	}
	msg := identity.CanonicalAuthMessage(req.DeviceID, req.Timestamp)
	if !ed25519.Verify(pub, msg, sig) {
		httpError(w, http.StatusUnauthorized, "signature verification failed")
		return
	}
	epoch := s.store.DeviceAuthEpoch(req.DeviceID)
	token, exp, err := issueSessionToken(req.DeviceID, epoch, s.cfg.ServerSecret)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "token issue failed")
		return
	}
	writeJSON(w, http.StatusOK, control.AuthResponse{
		Token:     token,
		ExpiresAt: exp.Format(time.RFC3339),
	})
}

func (s *Server) handlePeers(w http.ResponseWriter, r *http.Request, deviceID string) {
	dev, ok := s.store.GetDevice(deviceID)
	if !ok {
		httpError(w, http.StatusUnauthorized, "unknown device")
		return
	}
	peers := s.store.ListPeers(dev.NetworkID, deviceID)
	out := make([]control.PeerInfo, 0, len(peers))
	for _, p := range peers {
		out = append(out, control.PeerInfo{
			DeviceID: p.DeviceID,
			Name:     p.Name,
			Online:   s.hub.IsOnline(p.DeviceID),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request, deviceID string) {
	var req control.CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.PeerName == "" && req.PeerDevice == "" {
		httpError(w, http.StatusBadRequest, "peer_name or peer_device_id required")
		return
	}
	self, ok := s.store.GetDevice(deviceID)
	if !ok {
		httpError(w, http.StatusUnauthorized, "unknown device")
		return
	}
	var peer *DeviceRecord
	if req.PeerName != "" {
		p, ok := s.store.GetDeviceByName(req.PeerName)
		if !ok {
			httpError(w, http.StatusNotFound, "peer not found")
			return
		}
		peer = p
	} else {
		p, ok := s.store.GetDevice(req.PeerDevice)
		if !ok {
			httpError(w, http.StatusNotFound, "peer not found")
			return
		}
		peer = p
	}
	if peer.NetworkID != self.NetworkID {
		httpError(w, http.StatusForbidden, "peer not in network")
		return
	}
	sessionID := uuid.NewString()
	s.store.CreateSession(sessionID, deviceID, peer.DeviceID, self.NetworkID)

	readyInitiator := control.WSMessage{
		Type:      control.SignalSessionReady,
		SessionID: sessionID,
		Payload: control.SessionReadyPayload{
			SessionID:    sessionID,
			PeerDeviceID: peer.DeviceID,
			PeerName:     peer.Name,
			Role:         "controlling",
			Initiator:    deviceID,
		},
	}
	readyResponder := control.WSMessage{
		Type:      control.SignalSessionReady,
		SessionID: sessionID,
		Payload: control.SessionReadyPayload{
			SessionID:    sessionID,
			PeerDeviceID: deviceID,
			PeerName:     self.Name,
			Role:         "controlled",
			Initiator:    deviceID,
		},
	}
	_ = s.hub.SendTo(deviceID, readyInitiator)
	_ = s.hub.SendTo(peer.DeviceID, readyResponder)

	writeJSON(w, http.StatusCreated, control.CreateSessionResponse{
		SessionID:  sessionID,
		PeerDevice: peer.DeviceID,
		PeerName:   peer.Name,
		Role:       "controlling",
	})
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		// Development server; tighten for production deployments.
		return true
	},
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			token = strings.TrimPrefix(auth, "Bearer ")
		}
	}
	if token == "" {
		httpError(w, http.StatusUnauthorized, "missing token")
		return
	}
	deviceID, err := s.verifyDeviceBearer(token)
	if err != nil {
		httpError(w, http.StatusUnauthorized, "invalid token")
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Debug("websocket upgrade failed", "err", err)
		return
	}
	conn.SetReadLimit(maxWSMsgBytes)
	connID := s.hub.Register(deviceID, conn)
	s.log.Info("device online", "device_id", deviceID, "conn_id", connID)
	defer func() {
		s.hub.Unregister(deviceID, connID)
		_ = conn.Close()
		s.log.Info("device offline", "device_id", deviceID, "conn_id", connID)
	}()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				s.log.Debug("websocket read", "device", deviceID, "err", err)
			}
			return
		}
		var msg control.WSMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		switch msg.Type {
		case control.SignalICECredentials, control.SignalICECandidate, control.SignalGatherDone, control.SignalSessionClose:
			if msg.SessionID == "" {
				continue
			}
			if err := s.hub.RelaySignal(deviceID, msg.SessionID, msg, s.store); err != nil {
				s.log.Debug("signal relay failed", "device", deviceID, "err", err)
			}
		default:
			// Ignore unknown types.
		}
	}
}

type authedHandler func(http.ResponseWriter, *http.Request, string)

func (s *Server) withAuth(next authedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			httpError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		deviceID, err := s.verifyDeviceBearer(token)
		if err != nil {
			httpError(w, http.StatusUnauthorized, "invalid token")
			return
		}
		next(w, r, deviceID)
	}
}

func bearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

func httpError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// DrainBody closes request bodies in tests.
func DrainBody(r *http.Request) {
	if r.Body != nil {
		_, _ = io.Copy(io.Discard, r.Body)
	}
}
