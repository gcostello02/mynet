package server

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// CoordinatorIdentity is the long-lived Ed25519 identity of this coordinator network.
type CoordinatorIdentity struct {
	PublicKey   ed25519.PublicKey
	PrivateKey  ed25519.PrivateKey
	Fingerprint string // hex(SHA-256(public key))
}

func GenerateCoordinatorIdentity() (*CoordinatorIdentity, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &CoordinatorIdentity{
		PublicKey:   pub,
		PrivateKey:  priv,
		Fingerprint: fingerprintCoordinator(pub),
	}, nil
}

func fingerprintCoordinator(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return hex.EncodeToString(sum[:])
}

func coordinatorIdentityFromStored(pubB64 string) (*CoordinatorIdentity, error) {
	raw, err := base64.StdEncoding.DecodeString(pubB64)
	if err != nil {
		return nil, err
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid coordinator public key length")
	}
	pub := ed25519.PublicKey(raw)
	return &CoordinatorIdentity{
		PublicKey:   pub,
		Fingerprint: fingerprintCoordinator(pub),
	}, nil
}

func VerifyCoordinatorFingerprint(pubB64, wantFingerprint string) bool {
	id, err := coordinatorIdentityFromStored(pubB64)
	if err != nil {
		return false
	}
	return constantTimeSecretMatch(id.Fingerprint, stringsLowerHex(wantFingerprint))
}

func stringsLowerHex(s string) string {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'F' {
			c += 'a' - 'A'
		}
		out[i] = c
	}
	return string(out)
}
