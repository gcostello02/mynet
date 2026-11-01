package server

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var errSetupAlreadyDone = errors.New("setup already completed")

// firstAdminSetup runs admin creation and network initialization in one SQLite transaction.
func firstAdminSetup(db *sql.DB, m *NetworkMeta, username, passwordHash string, id *CoordinatorIdentity, displayName, publicURL, dataPath string) (*AdminRecord, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if complete, err := metaSetupCompleteTx(tx); err != nil {
		return nil, err
	} else if complete {
		return nil, errSetupAlreadyDone
	}
	var adminCount int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM admins`).Scan(&adminCount); err != nil {
		return nil, err
	}
	if adminCount > 0 {
		return nil, errSetupAlreadyDone
	}
	var deviceCount int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM devices`).Scan(&deviceCount); err != nil {
		return nil, err
	}
	if deviceCount > 0 {
		return nil, fmt.Errorf("existing enrolled devices")
	}

	admin, err := insertAdminTx(tx, username, passwordHash)
	if err != nil {
		return nil, err
	}
	if err := completeSetupTx(tx, m, id, displayName, publicURL); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if err := saveCoordinatorIdentity(dataPath, id); err != nil {
		return nil, err
	}
	return admin, nil
}

func metaSetupCompleteTx(tx *sql.Tx) (bool, error) {
	var v string
	err := tx.QueryRow(`SELECT value FROM network_meta WHERE key = ?`, metaSetupComplete).Scan(&v)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return v == "true", nil
}

func insertAdminTx(tx *sql.Tx, username, passwordHash string) (*AdminRecord, error) {
	rec := &AdminRecord{
		ID:           uuid.NewString(),
		Username:     username,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now().UTC(),
	}
	_, err := tx.Exec(`INSERT INTO admins (id, username, password_hash, created_at) VALUES (?, ?, ?, ?)`,
		rec.ID, rec.Username, rec.PasswordHash, rec.CreatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	return rec, nil
}

func completeSetupTx(tx *sql.Tx, m *NetworkMeta, id *CoordinatorIdentity, displayName, publicURL string) error {
	if err := setMetaValueTx(tx, metaSetupComplete, "true"); err != nil {
		return err
	}
	if err := setMetaValueTx(tx, metaLegacyEnrollmentEnabled, "false"); err != nil {
		return err
	}
	pubB64 := base64StdEncode(id.PublicKey)
	if err := setMetaValueTx(tx, metaCoordinatorPublicKey, pubB64); err != nil {
		return err
	}
	if err := setMetaValueTx(tx, metaCoordinatorFingerprint, id.Fingerprint); err != nil {
		return err
	}
	if displayName != "" {
		if err := setMetaValueTx(tx, metaNetworkDisplayName, displayName); err != nil {
			return err
		}
	}
	if publicURL != "" {
		if err := setMetaValueTx(tx, metaPublicServerURL, publicURL); err != nil {
			return err
		}
	}
	if err := setMetaValueTx(tx, metaBootstrapTokenHash, ""); err != nil {
		return err
	}
	if err := setMetaValueTx(tx, metaBootstrapExpiresAt, ""); err != nil {
		return err
	}
	m.mu.Lock()
	m.SetupComplete = true
	m.LegacyEnrollmentEnabled = false
	m.CoordinatorPublicKeyB64 = pubB64
	m.CoordinatorFingerprint = id.Fingerprint
	m.DisplayName = displayName
	m.PublicServerURL = publicURL
	m.mu.Unlock()
	return nil
}

func setMetaValueTx(tx *sql.Tx, key, value string) error {
	_, err := tx.Exec(`INSERT INTO network_meta (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}
