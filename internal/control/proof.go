package control

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"time"
)

func canonicalCoordinatorProofV1(networkID, fingerprint string, timestamp int64) []byte {
	return []byte(fmt.Sprintf("mynet-coordinator-proof-v1\n%s\n%s\n%d", networkID, fingerprint, timestamp))
}

func canonicalCoordinatorProofV2(networkID, fingerprint, challengeB64 string, timestamp int64) []byte {
	return []byte(fmt.Sprintf("mynet-coordinator-proof-v2\n%s\n%s\n%s\n%d", networkID, fingerprint, challengeB64, timestamp))
}

// VerifyCoordinatorProof validates v1 coordinator proof.
func VerifyCoordinatorProof(networkID, fingerprint, pubB64 string, timestamp int64, signatureB64 string, skew time.Duration) bool {
	pubRaw, err := base64.StdEncoding.DecodeString(pubB64)
	if err != nil || len(pubRaw) != ed25519.PublicKeySize {
		return false
	}
	now := time.Now().UTC().Unix()
	if timestamp < now-int64(skew.Seconds()) || timestamp > now+int64(skew.Seconds()) {
		return false
	}
	sig, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(pubRaw), canonicalCoordinatorProofV1(networkID, fingerprint, timestamp), sig)
}

// VerifyCoordinatorProofV2 validates challenge-bound coordinator proof.
func VerifyCoordinatorProofV2(networkID, fingerprint, challengeB64, pubB64 string, timestamp int64, signatureB64 string, skew time.Duration) bool {
	if challengeB64 == "" {
		return false
	}
	pubRaw, err := base64.StdEncoding.DecodeString(pubB64)
	if err != nil || len(pubRaw) != ed25519.PublicKeySize {
		return false
	}
	now := time.Now().UTC().Unix()
	if timestamp < now-int64(skew.Seconds()) || timestamp > now+int64(skew.Seconds()) {
		return false
	}
	sig, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(pubRaw), canonicalCoordinatorProofV2(networkID, fingerprint, challengeB64, timestamp), sig)
}
