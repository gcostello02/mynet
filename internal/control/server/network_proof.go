package server

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"
)

type networkProofRequest struct {
	Timestamp int64 `json:"timestamp"`
}

type networkProofResponse struct {
	NetworkID               string `json:"network_id"`
	CoordinatorFingerprint  string `json:"coordinator_fingerprint"`
	CoordinatorPublicKey    string `json:"coordinator_public_key"`
	Timestamp               int64  `json:"timestamp"`
	Signature               string `json:"signature"`
}

func (s *Server) handleNetworkProof(w http.ResponseWriter, r *http.Request) {
	meta := s.store.Meta().Snapshot()
	if !meta.SetupComplete {
		httpError(w, http.StatusNotFound, "network not initialized")
		return
	}
	if s.coordinatorIdentity == nil || len(s.coordinatorIdentity.PrivateKey) == 0 {
		httpError(w, http.StatusServiceUnavailable, "coordinator identity unavailable")
		return
	}
	var req networkProofRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := validateTimestamp(req.Timestamp, authTimeSkew); err != nil {
		httpError(w, http.StatusBadRequest, "invalid timestamp")
		return
	}
	msg := CanonicalCoordinatorProofMessage(s.store.NetworkID(), meta.CoordinatorFingerprint, req.Timestamp)
	sig := ed25519.Sign(s.coordinatorIdentity.PrivateKey, msg)
	writeJSON(w, http.StatusOK, networkProofResponse{
		NetworkID:              s.store.NetworkID(),
		CoordinatorFingerprint: meta.CoordinatorFingerprint,
		CoordinatorPublicKey:   meta.CoordinatorPublicKeyB64,
		Timestamp:              req.Timestamp,
		Signature:              base64.StdEncoding.EncodeToString(sig),
	})
}

// VerifyCoordinatorProof validates a coordinator proof for M2 clients.
func VerifyCoordinatorProof(networkID, fingerprint string, pubB64 string, timestamp int64, signatureB64 string, skew time.Duration) bool {
	pubRaw, err := base64.StdEncoding.DecodeString(pubB64)
	if err != nil || len(pubRaw) != ed25519.PublicKeySize {
		return false
	}
	if err := validateTimestamp(timestamp, skew); err != nil {
		return false
	}
	sig, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return false
	}
	msg := CanonicalCoordinatorProofMessage(networkID, fingerprint, timestamp)
	return ed25519.Verify(ed25519.PublicKey(pubRaw), msg, sig)
}
