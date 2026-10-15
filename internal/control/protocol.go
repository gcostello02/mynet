package control

// API version prefix for REST and WebSocket payloads.
const APIVersion = "v1"

// EnrollRequest enrolls a new device into the default network.
type EnrollRequest struct {
	DeviceID         string `json:"device_id"`
	Name             string `json:"name"`
	PublicKey        string `json:"public_key"`
	Timestamp        int64  `json:"timestamp"`
	Signature        string `json:"signature"` // base64
	EnrollmentSecret string `json:"enrollment_secret"`
}

// AuthRequest authenticates an already-enrolled device.
type AuthRequest struct {
	DeviceID  string `json:"device_id"`
	Timestamp int64  `json:"timestamp"`
	Signature string `json:"signature"` // base64
}

// AuthResponse returns a short-lived session bearer token.
type AuthResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

// PeerInfo is an authorized peer visible to the caller.
type PeerInfo struct {
	DeviceID string `json:"device_id"`
	Name     string `json:"name"`
	Online   bool   `json:"online"`
}

// CreateSessionRequest starts ICE signaling with a peer (by name or device id).
type CreateSessionRequest struct {
	PeerName   string `json:"peer_name,omitempty"`
	PeerDevice string `json:"peer_device_id,omitempty"`
}

// CreateSessionResponse identifies the signaling session.
type CreateSessionResponse struct {
	SessionID  string `json:"session_id"`
	PeerDevice string `json:"peer_device_id"`
	PeerName   string `json:"peer_name"`
	Role       string `json:"role"` // "controlling" or "controlled"
}

// WSMessage wraps signaling payloads delivered over WebSocket.
type WSMessage struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id,omitempty"`
	Payload   any    `json:"payload,omitempty"`
}

// Signal types (WSMessage.Type or nested discriminator).
const (
	SignalSessionReady   = "session_ready"
	SignalICECredentials = "ice_credentials"
	SignalICECandidate   = "ice_candidate"
	SignalGatherDone     = "gathering_complete"
	SignalSessionClose   = "session_close"
)

// SessionReadyPayload notifies both peers that signaling may begin.
type SessionReadyPayload struct {
	SessionID    string `json:"session_id"`
	PeerDeviceID string `json:"peer_device_id"`
	PeerName     string `json:"peer_name"`
	Role         string `json:"role"`
	Initiator    string `json:"initiator_device_id"`
}

// ICECredentialsPayload exchanges ICE ufrag/pwd for connectivity checks.
type ICECredentialsPayload struct {
	Ufrag string `json:"ufrag"`
	Pwd   string `json:"pwd"`
}

// ICECandidatePayload carries a marshaled ICE candidate string.
type ICECandidatePayload struct {
	Candidate string `json:"candidate"`
}

// VPNRegisterRequest registers a WireGuard public key for the authenticated device.
type VPNRegisterRequest struct {
	WGPublicKey string `json:"wg_public_key"` // hex, 32 bytes
	Timestamp   int64  `json:"timestamp"`
	Signature   string `json:"signature"` // base64 ed25519
}

// VPNPeerConfig is an authorized peer for tunnel setup.
type VPNPeerConfig struct {
	DeviceID    string   `json:"device_id"`
	Name        string   `json:"name"`
	VPNIPv4     string   `json:"vpn_ipv4"`
	WGPublicKey string   `json:"wg_public_key"`
	AllowedIPs  []string `json:"allowed_ips"`
	Revoked     bool     `json:"revoked,omitempty"`
}

// VPNRegisterResponse is returned after successful VPN registration.
type VPNRegisterResponse struct {
	VPNIPv4     string `json:"vpn_ipv4"`
	WGPublicKey string `json:"wg_public_key"`
}

// VPNConfigResponse is the caller's VPN assignment and peers.
type VPNConfigResponse struct {
	NetworkID   string          `json:"network_id"`
	Address     string          `json:"address"` // e.g. 10.77.0.1/32
	WGPublicKey string          `json:"wg_public_key"`
	Peers       []VPNPeerConfig `json:"peers"`
}
