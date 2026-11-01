package server

import (
	"database/sql"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

type InvitationRecord struct {
	ID        string
	TokenHash string
	Label     string
	ExpiresAt time.Time
	MaxUses   int
	Uses      int
	CreatedAt time.Time
	CreatedBy string
	RevokedAt time.Time
}

var (
	errInvitationInvalid   = errors.New("invitation invalid")
	errInvitationExpired   = errors.New("invitation expired")
	errInvitationExhausted = errors.New("invitation uses exhausted")
)

type invitationStore struct {
	mu   sync.Mutex
	byID map[string]*InvitationRecord
}

func newInvitationStore() *invitationStore {
	return &invitationStore{byID: make(map[string]*InvitationRecord)}
}

func loadInvitations(db *sql.DB, inv *invitationStore) error {
	rows, err := db.Query(`SELECT id, token_hash, label, expires_at, max_uses, uses, created_at, created_by, revoked_at FROM invitations`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var rec InvitationRecord
		var expStr, createdStr, revokedStr string
		if err := rows.Scan(&rec.ID, &rec.TokenHash, &rec.Label, &expStr, &rec.MaxUses, &rec.Uses, &createdStr, &rec.CreatedBy, &revokedStr); err != nil {
			return err
		}
		rec.ExpiresAt, _ = time.Parse(time.RFC3339Nano, expStr)
		rec.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdStr)
		if revokedStr != "" {
			rec.RevokedAt, _ = time.Parse(time.RFC3339Nano, revokedStr)
		}
		inv.byID[rec.ID] = &rec
	}
	return rows.Err()
}

func (s *Store) CreateInvitation(label string, expiresIn time.Duration, maxUses int, createdBy string) (id string, plaintextToken string, err error) {
	if maxUses < 1 {
		maxUses = 1
	}
	if expiresIn < time.Minute {
		expiresIn = time.Hour
	}
	token, err := newRandomToken(32)
	if err != nil {
		return "", "", err
	}
	id = uuid.NewString()
	rec := &InvitationRecord{
		ID:        id,
		TokenHash: hashInvitationToken(token),
		Label:     label,
		ExpiresAt: time.Now().UTC().Add(expiresIn),
		MaxUses:   maxUses,
		CreatedAt: time.Now().UTC(),
		CreatedBy: createdBy,
	}
	s.invitations.mu.Lock()
	s.invitations.byID[id] = rec
	s.invitations.mu.Unlock()
	if s.sqlDB != nil {
		if err := insertInvitation(s.sqlDB, rec); err != nil {
			return "", "", err
		}
	}
	return id, token, nil
}

func insertInvitation(db *sql.DB, rec *InvitationRecord) error {
	_, err := db.Exec(`INSERT INTO invitations (id, token_hash, label, expires_at, max_uses, uses, created_at, created_by, revoked_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, '')`,
		rec.ID, rec.TokenHash, rec.Label, rec.ExpiresAt.UTC().Format(time.RFC3339Nano),
		rec.MaxUses, rec.Uses, rec.CreatedAt.UTC().Format(time.RFC3339Nano), rec.CreatedBy)
	return err
}

func (s *Store) ListInvitations() []InvitationRecord {
	s.invitations.mu.Lock()
	defer s.invitations.mu.Unlock()
	out := make([]InvitationRecord, 0, len(s.invitations.byID))
	for _, rec := range s.invitations.byID {
		out = append(out, *rec)
	}
	return out
}

func (s *Store) RevokeInvitation(id string) bool {
	s.invitations.mu.Lock()
	rec, ok := s.invitations.byID[id]
	if !ok || !rec.RevokedAt.IsZero() {
		s.invitations.mu.Unlock()
		return false
	}
	rec.RevokedAt = time.Now().UTC()
	s.invitations.mu.Unlock()
	if s.sqlDB != nil {
		_, _ = s.sqlDB.Exec(`UPDATE invitations SET revoked_at=? WHERE id=?`, rec.RevokedAt.UTC().Format(time.RFC3339Nano), id)
	}
	return true
}

func invitationUsable(rec *InvitationRecord) error {
	if rec == nil {
		return errInvitationInvalid
	}
	if !rec.RevokedAt.IsZero() {
		return errInvitationInvalid
	}
	if time.Now().UTC().After(rec.ExpiresAt) {
		return errInvitationExpired
	}
	if rec.Uses >= rec.MaxUses {
		return errInvitationExhausted
	}
	return nil
}

// CheckInvitation validates token without consuming a use.
func (s *Store) CheckInvitation(plaintextToken string) error {
	hash := hashInvitationToken(plaintextToken)
	s.invitations.mu.Lock()
	defer s.invitations.mu.Unlock()
	for _, rec := range s.invitations.byID {
		if rec.TokenHash == hash {
			return invitationUsable(rec)
		}
	}
	return errInvitationInvalid
}

// RedeemInvitation atomically validates and consumes one use of the token.
func (s *Store) RedeemInvitation(plaintextToken string) error {
	hash := hashInvitationToken(plaintextToken)
	s.invitations.mu.Lock()
	var id string
	var usesAfter int
	for _, rec := range s.invitations.byID {
		if rec.TokenHash != hash {
			continue
		}
		if err := invitationUsable(rec); err != nil {
			s.invitations.mu.Unlock()
			return err
		}
		rec.Uses++
		id = rec.ID
		usesAfter = rec.Uses
		break
	}
	if id == "" {
		s.invitations.mu.Unlock()
		return errInvitationInvalid
	}
	s.invitations.mu.Unlock()

	if s.sqlDB != nil {
		res, err := s.sqlDB.Exec(`UPDATE invitations SET uses=uses+1 WHERE id=? AND uses < max_uses AND revoked_at='' AND expires_at > ?`,
			id, time.Now().UTC().Format(time.RFC3339Nano))
		if err != nil {
			s.invitations.mu.Lock()
			if rec := s.invitations.byID[id]; rec != nil && rec.Uses == usesAfter {
				rec.Uses--
			}
			s.invitations.mu.Unlock()
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			s.invitations.mu.Lock()
			if rec := s.invitations.byID[id]; rec != nil && rec.Uses == usesAfter {
				rec.Uses--
			}
			s.invitations.mu.Unlock()
			return errInvitationExhausted
		}
	}
	return nil
}
