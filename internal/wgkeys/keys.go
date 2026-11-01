package wgkeys

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/curve25519"
)

const fileVersion = 1

// Identity is a persistent WireGuard key pair (separate from Ed25519 device identity).
type Identity struct {
	Version    int    `json:"version"`
	PublicKey  string `json:"public_key"`  // base64 (32 bytes)
	PrivateKey string `json:"private_key"` // base64 (32 bytes)
	CreatedAt  string `json:"created_at"`
	VPNIPv4    string `json:"vpn_ipv4,omitempty"` // last server-assigned address
}

func Path(stateDir string) string {
	return filepath.Join(stateDir, "wg.json")
}

// Load reads wg.json or returns an error if missing.
func Load(stateDir string) (*Identity, error) {
	path := Path(stateDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var id Identity
	if err := json.Unmarshal(data, &id); err != nil {
		return nil, err
	}
	if err := validate(&id); err != nil {
		return nil, err
	}
	return &id, nil
}

// LoadOrCreate reads wg.json or generates a new key pair.
func LoadOrCreate(stateDir string) (*Identity, error) {
	path := Path(stateDir)
	data, err := os.ReadFile(path)
	if err == nil {
		var id Identity
		if err := json.Unmarshal(data, &id); err != nil {
			return nil, err
		}
		if id.Version != fileVersion {
			return nil, fmt.Errorf("unsupported wg identity version %d", id.Version)
		}
		if err := validate(&id); err != nil {
			return nil, err
		}
		return &id, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	var priv, pub [32]byte
	if _, err := rand.Read(priv[:]); err != nil {
		return nil, err
	}
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64
	curve25519.ScalarBaseMult(&pub, &priv)
	id := &Identity{
		Version:    fileVersion,
		PrivateKey: base64.StdEncoding.EncodeToString(priv[:]),
		PublicKey:  base64.StdEncoding.EncodeToString(pub[:]),
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	if err := Save(stateDir, id); err != nil {
		return nil, err
	}
	return id, nil
}

func validate(id *Identity) error {
	priv, err := base64.StdEncoding.DecodeString(id.PrivateKey)
	if err != nil || len(priv) != 32 {
		return errors.New("invalid wg private key")
	}
	pub, err := base64.StdEncoding.DecodeString(id.PublicKey)
	if err != nil || len(pub) != 32 {
		return errors.New("invalid wg public key")
	}
	var sk, derived [32]byte
	copy(sk[:], priv)
	curve25519.ScalarBaseMult(&derived, &sk)
	if base64.StdEncoding.EncodeToString(derived[:]) != id.PublicKey {
		return errors.New("wg public key does not match private key")
	}
	return nil
}

func Save(stateDir string, id *Identity) error {
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(id, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(Path(stateDir), data, 0o600)
}

func (id *Identity) PrivateKeyBytes() ([]byte, error) {
	return base64.StdEncoding.DecodeString(id.PrivateKey)
}

func (id *Identity) PublicKeyBytes() ([]byte, error) {
	return base64.StdEncoding.DecodeString(id.PublicKey)
}

func (id *Identity) PublicKeyHex() (string, error) {
	pub, err := id.PublicKeyBytes()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", pub), nil
}

// SaveAssignedVPNIPv4 records the coordination server's assigned address in wg.json.
// Clear removes persisted WireGuard keys (used when replacing device identity).
func Clear(stateDir string) error {
	err := os.Remove(Path(stateDir))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func SaveAssignedVPNIPv4(stateDir string, vpnIPv4 string) error {
	if vpnIPv4 == "" {
		return nil
	}
	id, err := LoadOrCreate(stateDir)
	if err != nil {
		return err
	}
	if id.VPNIPv4 == vpnIPv4 {
		return nil
	}
	id.VPNIPv4 = vpnIPv4
	return Save(stateDir, id)
}
