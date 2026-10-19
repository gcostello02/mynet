package server

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

func loadAdmins(db *sql.DB) (map[string]*AdminRecord, error) {
	rows, err := db.Query(`SELECT id, username, password_hash, created_at FROM admins`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]*AdminRecord)
	for rows.Next() {
		var rec AdminRecord
		var createdStr string
		if err := rows.Scan(&rec.ID, &rec.Username, &rec.PasswordHash, &createdStr); err != nil {
			return nil, err
		}
		rec.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdStr)
		out[rec.ID] = &rec
	}
	return out, rows.Err()
}

func insertAdmin(db *sql.DB, username, passwordHash string) (*AdminRecord, error) {
	rec := &AdminRecord{
		ID:           uuid.NewString(),
		Username:     username,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now().UTC(),
	}
	_, err := db.Exec(`INSERT INTO admins (id, username, password_hash, created_at) VALUES (?, ?, ?, ?)`,
		rec.ID, rec.Username, rec.PasswordHash, rec.CreatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	return rec, nil
}

func countAdmins(db *sql.DB) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM admins`).Scan(&n)
	return n, err
}
