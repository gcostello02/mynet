package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type AdminSession struct {
	ServerURL    string `json:"server_url"`
	SessionToken string `json:"session_token"`
	ExpiresAt    string `json:"expires_at,omitempty"`
}

func AdminSessionPath(stateDir string) string {
	return filepath.Join(stateDir, "admin-session.json")
}

func LoadAdminSession(stateDir string) (AdminSession, error) {
	data, err := os.ReadFile(AdminSessionPath(stateDir))
	if err != nil {
		return AdminSession{}, err
	}
	var s AdminSession
	if err := json.Unmarshal(data, &s); err != nil {
		return AdminSession{}, err
	}
	if s.SessionToken == "" {
		return AdminSession{}, errors.New("empty admin session")
	}
	return s, nil
}

func SaveAdminSession(stateDir string, s AdminSession) error {
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(AdminSessionPath(stateDir), data, 0o600)
}

func ClearAdminSession(stateDir string) error {
	err := os.Remove(AdminSessionPath(stateDir))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
