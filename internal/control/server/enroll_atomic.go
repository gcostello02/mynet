package server

import (
	"context"
	"database/sql"
	"time"
)

// EnrollDeviceWithInvitationAtomic redeems the invitation and inserts the device in one DB transaction.
// In-memory-only stores (no sqlDB) use a single store mutex; see docs/m1-coordinator-admin.md.
func (s *Store) EnrollDeviceWithInvitationAtomic(plaintextToken string, rec *DeviceRecord) error {
	if rec == nil || rec.Revoked {
		return errRevoked
	}
	if s.sqlDB == nil {
		return s.enrollWithInvitationMemory(plaintextToken, rec)
	}
	hash := hashInvitationToken(plaintextToken)
	ctx := context.Background()
	tx, err := s.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339Nano)

	var invID string
	var uses, maxUses int
	var expiresAt, revokedAt string
	err = tx.QueryRow(`SELECT id, uses, max_uses, expires_at, revoked_at FROM invitations WHERE token_hash=?`, hash).
		Scan(&invID, &uses, &maxUses, &expiresAt, &revokedAt)
	if err == sql.ErrNoRows {
		return errInvitationInvalid
	}
	if err != nil {
		return err
	}
	if revokedAt != "" {
		return errInvitationInvalid
	}
	exp, err := time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil || now.After(exp) {
		return errInvitationExpired
	}
	if uses >= maxUses {
		return errInvitationExhausted
	}

	var existingRevoked int
	var existingPub string
	deviceExists := false
	err = tx.QueryRow(`SELECT revoked, public_key FROM devices WHERE device_id = ?`, rec.DeviceID).Scan(&existingRevoked, &existingPub)
	switch {
	case err == sql.ErrNoRows:
		err = nil
	case err != nil:
		return err
	default:
		deviceExists = true
		if existingRevoked != 0 {
			return errRevoked
		}
		if existingPub != rec.PublicKey {
			return errDeviceExists
		}
	}

	var nameOwner string
	err = tx.QueryRow(`SELECT device_id FROM devices WHERE name = ?`, rec.Name).Scan(&nameOwner)
	if err == nil && nameOwner != rec.DeviceID {
		return errNameTaken
	}
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	res, err := tx.Exec(`UPDATE invitations SET uses = uses + 1 WHERE id = ? AND uses < max_uses AND revoked_at = '' AND expires_at > ?`,
		invID, nowStr)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errInvitationExhausted
	}

	enrolled := rec.Enrolled.UTC().Format(time.RFC3339Nano)
	revoked := 0
	if !deviceExists {
		_, err = tx.Exec(`INSERT INTO devices (device_id, name, public_key, network_id, revoked, enrolled_at, wg_public_key, vpn_ipv4, auth_epoch, renamed_at)
			VALUES (?, ?, ?, ?, ?, ?, '', '', 0, '')`,
			rec.DeviceID, rec.Name, rec.PublicKey, rec.NetworkID, revoked, enrolled)
	} else {
		_, err = tx.Exec(`UPDATE devices SET name = ?, public_key = ?, network_id = ? WHERE device_id = ? AND revoked = 0`,
			rec.Name, rec.PublicKey, rec.NetworkID, rec.DeviceID)
	}
	if err != nil {
		if isUniqueConstraint(err) {
			return errNameTaken
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	s.applyEnrolledDeviceMemory(rec, invID, uses+1)
	return nil
}

func isUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return stringsContains(msg, "UNIQUE constraint") || stringsContains(msg, "constraint failed")
}

func (s *Store) applyEnrolledDeviceMemory(rec *DeviceRecord, invID string, newUses int) {
	s.mu.Lock()
	if existing, ok := s.devices[rec.DeviceID]; ok {
		if existing.Revoked {
			s.mu.Unlock()
			return
		}
		delete(s.byName, existing.Name)
		existing.Name = rec.Name
		existing.PublicKey = rec.PublicKey
		s.byName[rec.Name] = rec.DeviceID
	} else {
		cp := *rec
		s.devices[rec.DeviceID] = &cp
		s.byName[rec.Name] = rec.DeviceID
	}
	s.mu.Unlock()

	s.invitations.mu.Lock()
	if inv, ok := s.invitations.byID[invID]; ok {
		inv.Uses = newUses
	}
	s.invitations.mu.Unlock()
}

func (s *Store) enrollWithInvitationMemory(plaintextToken string, rec *DeviceRecord) error {
	hash := hashInvitationToken(plaintextToken)
	s.mu.Lock()
	defer s.mu.Unlock()

	if rec.Revoked {
		return errRevoked
	}
	if existing, ok := s.devices[rec.DeviceID]; ok {
		if existing.Revoked {
			return errRevoked
		}
		if existing.PublicKey != rec.PublicKey {
			return errDeviceExists
		}
	}
	if other, ok := s.byName[rec.Name]; ok && other != rec.DeviceID {
		return errNameTaken
	}

	s.invitations.mu.Lock()
	var invID string
	var target *InvitationRecord
	for _, recInv := range s.invitations.byID {
		if recInv.TokenHash == hash {
			target = recInv
			invID = recInv.ID
			break
		}
	}
	if err := invitationUsable(target); err != nil {
		s.invitations.mu.Unlock()
		return err
	}
	target.Uses++
	newUses := target.Uses
	s.invitations.mu.Unlock()

	if existing, ok := s.devices[rec.DeviceID]; ok {
		delete(s.byName, existing.Name)
		existing.Name = rec.Name
		s.byName[rec.Name] = rec.DeviceID
	} else {
		cp := *rec
		s.devices[rec.DeviceID] = &cp
		s.byName[rec.Name] = rec.DeviceID
	}
	_ = invID
	_ = newUses
	return nil
}
