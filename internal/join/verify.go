package join

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/gcostello02/mynet/internal/control"
	"github.com/gcostello02/mynet/internal/invitation"
)

const proofSkew = 5 * time.Minute

var (
	ErrProofDowngrade = errors.New("coordinator did not return challenge-bound proof v2")
	ErrProofInvalid   = errors.New("coordinator proof signature invalid")
)

// VerifyCoordinator checks invitation pins against live coordinator metadata and proof.
func VerifyCoordinator(inv invitation.Invite) error {
	api, err := control.NewSecureAPIClient(inv.ServerURL)
	if err != nil {
		return err
	}
	pub, err := api.NetworkPublic()
	if err != nil {
		return fmt.Errorf("fetch network public metadata: %w", err)
	}
	if pub.NetworkID != inv.NetworkID {
		return fmt.Errorf("network_id mismatch")
	}
	if !fingerprintEqual(pub.CoordinatorFingerprint, inv.CoordinatorFingerprint) {
		return fmt.Errorf("coordinator fingerprint mismatch")
	}
	pinnedPub := inv.CoordinatorPublicKey
	if pinnedPub == "" {
		pinnedPub = pub.CoordinatorPublicKey
	} else if pinnedPub != pub.CoordinatorPublicKey {
		return fmt.Errorf("coordinator public key mismatch")
	}
	challenge := make([]byte, 32)
	if _, err := rand.Read(challenge); err != nil {
		return err
	}
	challengeB64 := base64.RawURLEncoding.EncodeToString(challenge)
	ts := time.Now().UTC().Unix()
	proof, err := api.NetworkProof(ts, challengeB64)
	if err != nil {
		return fmt.Errorf("coordinator proof: %w", err)
	}
	if proof.ProofVersion < 2 {
		return ErrProofDowngrade
	}
	if proof.Challenge != challengeB64 {
		return fmt.Errorf("coordinator proof challenge mismatch")
	}
	if proof.NetworkID != inv.NetworkID || !fingerprintEqual(proof.CoordinatorFingerprint, inv.CoordinatorFingerprint) {
		return fmt.Errorf("coordinator proof metadata mismatch")
	}
	if proof.CoordinatorPublicKey != pinnedPub {
		return fmt.Errorf("coordinator proof public key mismatch")
	}
	if !control.VerifyCoordinatorProofV2(proof.NetworkID, proof.CoordinatorFingerprint, proof.Challenge, pinnedPub, proof.Timestamp, proof.Signature, proofSkew) {
		return ErrProofInvalid
	}
	return nil
}

func fingerprintEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'F' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'F' {
			cb += 'a' - 'A'
		}
		v |= ca ^ cb
	}
	return v == 0
}
