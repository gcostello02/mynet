package server

import (
	"database/sql"
	"encoding/base64"
	"sync"
)

const (
	metaSetupComplete           = "setup_complete"
	metaCoordinatorPublicKey    = "coordinator_public_key"
	metaCoordinatorFingerprint  = "coordinator_fingerprint"
	metaNetworkDisplayName      = "network_display_name"
	metaPublicServerURL         = "public_server_url"
	metaLegacyEnrollmentEnabled = "legacy_enrollment_enabled"
)

// NetworkMeta holds coordinator-wide settings loaded from SQLite or memory.
type NetworkMeta struct {
	mu sync.RWMutex

	SetupComplete           bool
	CoordinatorPublicKeyB64 string
	CoordinatorFingerprint  string
	DisplayName             string
	PublicServerURL         string
	LegacyEnrollmentEnabled bool
}

func (m *NetworkMeta) Snapshot() NetworkMeta {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return NetworkMeta{
		SetupComplete:           m.SetupComplete,
		CoordinatorPublicKeyB64: m.CoordinatorPublicKeyB64,
		CoordinatorFingerprint:  m.CoordinatorFingerprint,
		DisplayName:             m.DisplayName,
		PublicServerURL:         m.PublicServerURL,
		LegacyEnrollmentEnabled: m.LegacyEnrollmentEnabled,
	}
}

func loadNetworkMeta(db *sql.DB) (*NetworkMeta, error) {
	m := &NetworkMeta{}
	rows, err := db.Query(`SELECT key, value FROM network_meta`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	kv := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		kv[k] = v
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	m.SetupComplete = kv[metaSetupComplete] == "true"
	m.CoordinatorPublicKeyB64 = kv[metaCoordinatorPublicKey]
	m.CoordinatorFingerprint = kv[metaCoordinatorFingerprint]
	m.DisplayName = kv[metaNetworkDisplayName]
	m.PublicServerURL = kv[metaPublicServerURL]
	m.LegacyEnrollmentEnabled = kv[metaLegacyEnrollmentEnabled] == "true"
	return m, nil
}

func setMetaValue(db *sql.DB, key, value string) error {
	_, err := db.Exec(`INSERT INTO network_meta (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

func completeSetup(db *sql.DB, m *NetworkMeta, id *CoordinatorIdentity, displayName, publicURL, dataPath string) error {
	if err := setMetaValue(db, metaSetupComplete, "true"); err != nil {
		return err
	}
	// New networks default to invitation-only until admin enables legacy.
	if err := setMetaValue(db, metaLegacyEnrollmentEnabled, "false"); err != nil {
		return err
	}
	m.mu.Lock()
	m.LegacyEnrollmentEnabled = false
	m.mu.Unlock()
	pubB64 := base64StdEncode(id.PublicKey)
	if err := setMetaValue(db, metaCoordinatorPublicKey, pubB64); err != nil {
		return err
	}
	if err := setMetaValue(db, metaCoordinatorFingerprint, id.Fingerprint); err != nil {
		return err
	}
	if displayName != "" {
		if err := setMetaValue(db, metaNetworkDisplayName, displayName); err != nil {
			return err
		}
	}
	if publicURL != "" {
		if err := setMetaValue(db, metaPublicServerURL, publicURL); err != nil {
			return err
		}
	}
	m.mu.Lock()
	m.SetupComplete = true
	m.CoordinatorPublicKeyB64 = pubB64
	m.CoordinatorFingerprint = id.Fingerprint
	m.DisplayName = displayName
	m.PublicServerURL = publicURL
	m.mu.Unlock()
	if err := saveCoordinatorIdentity(dataPath, id); err != nil {
		return err
	}
	return nil
}

func enableLegacyEnrollmentMeta(db *sql.DB, m *NetworkMeta) error {
	if err := setMetaValue(db, metaLegacyEnrollmentEnabled, "true"); err != nil {
		return err
	}
	m.mu.Lock()
	m.LegacyEnrollmentEnabled = true
	m.mu.Unlock()
	return nil
}

func base64StdEncode(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}
