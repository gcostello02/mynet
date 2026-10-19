package server

import (
	"errors"
)

// validateEnrollmentGate checks credentials without consuming invitation uses.
func (s *Server) validateEnrollmentGate(req controlEnrollFields) error {
	meta := s.store.Meta().Snapshot()
	if req.EnrollmentToken != "" {
		if !meta.SetupComplete {
			return errors.New("network not initialized")
		}
		if req.CoordinatorFingerprint == "" {
			return errors.New("coordinator_fingerprint required")
		}
		if !VerifyCoordinatorFingerprint(meta.CoordinatorPublicKeyB64, req.CoordinatorFingerprint) {
			return errors.New("coordinator fingerprint mismatch")
		}
		return s.store.CheckInvitation(req.EnrollmentToken)
	}
	if meta.SetupComplete && !meta.LegacyEnrollmentEnabled {
		return errors.New("invitation required")
	}
	if req.EnrollmentSecret == "" {
		return errors.New("enrollment credentials required")
	}
	gotHash := hashEnrollmentSecret(req.EnrollmentSecret)
	if !constantTimeSecretMatch(gotHash, s.store.EnrollmentSecretHash()) {
		return errors.New("enrollment denied")
	}
	return nil
}
