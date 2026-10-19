package server

import (
	"database/sql"
	"fmt"
)

const schemaVersion = 2

const metaSchema = `
CREATE TABLE IF NOT EXISTS network_meta (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
`

const adminsSchema = `
CREATE TABLE IF NOT EXISTS admins (
  id TEXT PRIMARY KEY,
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  created_at TEXT NOT NULL
);
`

const invitationsSchema = `
CREATE TABLE IF NOT EXISTS invitations (
  id TEXT PRIMARY KEY,
  token_hash TEXT NOT NULL UNIQUE,
  label TEXT NOT NULL DEFAULT '',
  expires_at TEXT NOT NULL,
  max_uses INTEGER NOT NULL,
  uses INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  created_by TEXT NOT NULL DEFAULT '',
  revoked_at TEXT NOT NULL DEFAULT ''
);
`

const adminSessionsSchema = `
CREATE TABLE IF NOT EXISTS admin_sessions (
  id TEXT PRIMARY KEY,
  admin_id TEXT NOT NULL,
  csrf_token_hash TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL
);
`

func migrateCoordinatorDB(db *sql.DB) error {
	if err := migrateDevicesDB(db); err != nil {
		return err
	}
	for _, q := range []string{metaSchema, adminsSchema, invitationsSchema, adminSessionsSchema} {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	deviceCols := []string{
		`ALTER TABLE devices ADD COLUMN auth_epoch INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE devices ADD COLUMN renamed_at TEXT NOT NULL DEFAULT ''`,
	}
	for _, q := range deviceCols {
		_, err := db.Exec(q)
		if err != nil && !isDuplicateColumn(err) {
			return err
		}
	}
	if _, err := db.Exec(`INSERT OR IGNORE INTO network_meta (key, value) VALUES ('schema_version', ?)`, fmt.Sprintf("%d", schemaVersion)); err != nil {
		return err
	}
	return nil
}

func openCoordinatorDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := migrateCoordinatorDB(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
