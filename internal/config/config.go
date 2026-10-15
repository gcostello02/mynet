package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// DefaultStateDir is the per-user MyNet configuration directory.
func DefaultStateDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".mynet"
	}
	return filepath.Join(home, ".mynet")
}

// ClientConfig holds client-side settings (no secrets beyond paths).
type ClientConfig struct {
	ServerURL string   `json:"server_url"`
	STUNURLs  []string `json:"stun_urls"`
	TURNURLs  []string `json:"turn_urls"`
	// ICEURLs, if non-empty, replaces the combined stun_urls + turn_urls list.
	ICEURLs                  []string      `json:"ice_urls,omitempty"`
	AllowTailscaleCandidates bool          `json:"allow_tailscale_candidates,omitempty"`
	Network                  NetworkConfig `json:"network,omitempty"`
}

// DefaultClientConfig returns development-friendly defaults.
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		ServerURL: "http://127.0.0.1:8080",
		STUNURLs:  []string{"stun:stun.l.google.com:19302"},
	}
}

func IdentityPath(stateDir string) string {
	return filepath.Join(stateDir, "identity.json")
}

func ClientConfigPath(stateDir string) string {
	return filepath.Join(stateDir, "config.json")
}

func SessionPath(stateDir string) string {
	return filepath.Join(stateDir, "session.json")
}

// LoadClientConfig reads client config or returns defaults if missing.
func LoadClientConfig(stateDir string) (ClientConfig, error) {
	path := ClientConfigPath(stateDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultClientConfig(), nil
		}
		return ClientConfig{}, err
	}
	var cfg ClientConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ClientConfig{}, err
	}
	if cfg.ServerURL == "" {
		cfg.ServerURL = DefaultClientConfig().ServerURL
	}
	if len(cfg.STUNURLs) == 0 {
		cfg.STUNURLs = DefaultClientConfig().STUNURLs
	}
	return cfg, nil
}

// SaveClientConfig writes client config.
func SaveClientConfig(stateDir string, cfg ClientConfig) error {
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(ClientConfigPath(stateDir), data, 0o600)
}

// SessionToken stores the bearer token from login (not the enrollment secret).
type SessionToken struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

func LoadSessionToken(stateDir string) (*SessionToken, error) {
	data, err := os.ReadFile(SessionPath(stateDir))
	if err != nil {
		return nil, err
	}
	var tok SessionToken
	if err := json.Unmarshal(data, &tok); err != nil {
		return nil, err
	}
	return &tok, nil
}

func SaveSessionToken(stateDir string, tok SessionToken) error {
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(tok, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(SessionPath(stateDir), data, 0o600)
}
