package server

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const devicesSchema = `
CREATE TABLE IF NOT EXISTS devices (
  device_id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  public_key TEXT NOT NULL,
  network_id TEXT NOT NULL,
  revoked INTEGER NOT NULL DEFAULT 0,
  enrolled_at TEXT NOT NULL,
  wg_public_key TEXT NOT NULL DEFAULT '',
  vpn_ipv4 TEXT NOT NULL DEFAULT ''
);
`

func migrateDevicesDB(db *sql.DB) error {
	if _, err := db.Exec(devicesSchema); err != nil {
		return err
	}
	cols := []string{
		`ALTER TABLE devices ADD COLUMN wg_public_key TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE devices ADD COLUMN vpn_ipv4 TEXT NOT NULL DEFAULT ''`,
	}
	for _, q := range cols {
		_, err := db.Exec(q)
		if err != nil && !isDuplicateColumn(err) {
			return err
		}
	}
	return nil
}

func isDuplicateColumn(err error) bool {
	if err == nil {
		return false
	}
	return stringsContains(err.Error(), "duplicate column")
}

func stringsContains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// LoadDevicesFromSQLite loads enrolled devices into an in-memory store.
func LoadDevicesFromSQLite(path string, store *Store) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := migrateCoordinatorDB(db); err != nil {
		return err
	}
	rows, err := db.Query(`SELECT device_id, name, public_key, network_id, revoked, enrolled_at, wg_public_key, vpn_ipv4, auth_epoch, renamed_at FROM devices`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var rec DeviceRecord
		var revoked int
		var enrolledStr, renamedStr string
		if err := rows.Scan(&rec.DeviceID, &rec.Name, &rec.PublicKey, &rec.NetworkID, &revoked, &enrolledStr, &rec.WGPublicKey, &rec.VPNIPv4, &rec.AuthEpoch, &renamedStr); err != nil {
			return err
		}
		rec.Revoked = revoked != 0
		if t, err := time.Parse(time.RFC3339Nano, enrolledStr); err == nil {
			rec.Enrolled = t
		}
		if renamedStr != "" {
			if t, err := time.Parse(time.RFC3339Nano, renamedStr); err == nil {
				rec.RenamedAt = t
			}
		}
		store.mu.Lock()
		store.devices[rec.DeviceID] = &rec
		if !rec.Revoked {
			store.byName[rec.Name] = rec.DeviceID
		}
		store.mu.Unlock()
	}
	return rows.Err()
}

// PersistDevice saves or updates a device record.
func PersistDevice(path string, rec *DeviceRecord) error {
	if path == "" || rec == nil {
		return nil
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := migrateCoordinatorDB(db); err != nil {
		return err
	}
	revoked := 0
	if rec.Revoked {
		revoked = 1
	}
	enrolled := rec.Enrolled.UTC().Format(time.RFC3339Nano)
	renamed := ""
	if !rec.RenamedAt.IsZero() {
		renamed = rec.RenamedAt.UTC().Format(time.RFC3339Nano)
	}
	_, err = db.Exec(`INSERT INTO devices (device_id, name, public_key, network_id, revoked, enrolled_at, wg_public_key, vpn_ipv4, auth_epoch, renamed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(device_id) DO UPDATE SET name=excluded.name, public_key=excluded.public_key, network_id=excluded.network_id, revoked=excluded.revoked, enrolled_at=excluded.enrolled_at, wg_public_key=excluded.wg_public_key, vpn_ipv4=excluded.vpn_ipv4, auth_epoch=excluded.auth_epoch, renamed_at=excluded.renamed_at`,
		rec.DeviceID, rec.Name, rec.PublicKey, rec.NetworkID, revoked, enrolled, rec.WGPublicKey, rec.VPNIPv4, rec.AuthEpoch, renamed)
	return err
}

func persistDeviceRevoked(path string, rec *DeviceRecord) error {
	if path == "" || rec == nil {
		return nil
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := migrateCoordinatorDB(db); err != nil {
		return err
	}
	renamed := ""
	if !rec.RenamedAt.IsZero() {
		renamed = rec.RenamedAt.UTC().Format(time.RFC3339Nano)
	}
	res, err := db.Exec(`UPDATE devices SET revoked=1, auth_epoch=?, renamed_at=? WHERE device_id=?`,
		rec.AuthEpoch, renamed, rec.DeviceID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("device not found in database")
	}
	return nil
}
