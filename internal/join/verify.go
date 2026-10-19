package join

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/grantcostello/mynet/internal/control"
	"github.com/grantcostello/mynet/internal/invitation"
)

const proofSkew = 5 * time.Minute

// VerifyCoordinator checks invitation pins against live coordinator metadata and proof.
func VerifyCoordinator(inv invitation.Invite) error {
	api := control.NewSecureAPIClient(inv.ServerURL)
	pub, err := api.NetworkPublic()
	if err != nil {
		return fmt.Errorf("fetch network public metadata: %w", err)
	}
	if pub.NetworkID != inv.NetworkID {
		return fmt.Errorf("network_id mismatch: invitation %q, coordinator %q", inv.NetworkID, pub.NetworkID)
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
	if proof.Challenge != challengeB64 {
		return fmt.Errorf("coordinator proof challenge mismatch")
	}
	if proof.NetworkID != inv.NetworkID || !fingerprintEqual(proof.CoordinatorFingerprint, inv.CoordinatorFingerprint) {
		return fmt.Errorf("coordinator proof metadata mismatch")
	}
	if proof.ProofVersion >= 2 {
		if !control.VerifyCoordinatorProofV2(proof.NetworkID, proof.CoordinatorFingerprint, proof.Challenge, proof.CoordinatorPublicKey, proof.Timestamp, proof.Signature, proofSkew) {
			return fmt.Errorf("coordinator proof signature invalid")
		}
	} else if !control.VerifyCoordinatorProof(proof.NetworkID, proof.CoordinatorFingerprint, proof.CoordinatorPublicKey, proof.Timestamp, proof.Signature, proofSkew) {
		return fmt.Errorf("coordinator proof signature invalid")
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
