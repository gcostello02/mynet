package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const currentVersion = 1

// DeviceIdentity is the long-lived cryptographic identity for a MyNet device.
type DeviceIdentity struct {
	Version    int    `json:"version"`
	Name       string `json:"name"`
	PublicKey  string `json:"public_key"`  // base64 raw ed25519 public key
	PrivateKey string `json:"private_key"` // base64 raw ed25519 private key
	DeviceID   string `json:"device_id"`
	CreatedAt  string `json:"created_at"`
}

// PublicKeyBytes returns the decoded public key.
func (d *DeviceIdentity) PublicKeyBytes() ([]byte, error) {
	b, err := base64.StdEncoding.DecodeString(d.PublicKey)
	if err != nil {
		return nil, err
	}
	if len(b) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key length %d", len(b))
	}
	return b, nil
}

// PrivateKeySeed returns the 32-byte seed for ed25519 signing.
func (d *DeviceIdentity) PrivateKeySeed() ([]byte, error) {
	b, err := base64.StdEncoding.DecodeString(d.PrivateKey)
	if err != nil {
		return nil, err
	}
	if len(b) == ed25519.SeedSize {
		return b, nil
	}
	if len(b) == ed25519.PrivateKeySize {
		return ed25519.PrivateKey(b).Seed(), nil
	}
	return nil, fmt.Errorf("invalid private key length %d", len(b))
}

// Sign signs a message with this device's private key.
func (d *DeviceIdentity) Sign(message []byte) ([]byte, error) {
	seed, err := d.PrivateKeySeed()
	if err != nil {
		return nil, err
	}
	priv := ed25519.NewKeyFromSeed(seed)
	sig := ed25519.Sign(priv, message)
	return sig, nil
}

// DeviceIDFromPublicKey derives a stable device identifier from a public key.
func DeviceIDFromPublicKey(pub []byte) string {
	sum := sha256.Sum256(pub)
	return hex.EncodeToString(sum[:16])
}

// Generate creates a new device identity with the given friendly name.
func Generate(name string) (*DeviceIdentity, error) {
	if name == "" {
		return nil, errors.New("device name is required")
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	seed := priv.Seed()
	return &DeviceIdentity{
		Version:    currentVersion,
		Name:       name,
		PublicKey:  base64.StdEncoding.EncodeToString(pub),
		PrivateKey: base64.StdEncoding.EncodeToString(seed),
		DeviceID:   DeviceIDFromPublicKey(pub),
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// Save writes identity to path with restrictive permissions.
func Save(path string, id *DeviceIdentity) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(id, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	return nil
}

// Load reads identity from path.
func Load(path string) (*DeviceIdentity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var id DeviceIdentity
	if err := json.Unmarshal(data, &id); err != nil {
		return nil, err
	}
	if id.Version != currentVersion {
		return nil, fmt.Errorf("unsupported identity version %d", id.Version)
	}
	pub, err := id.PublicKeyBytes()
	if err != nil {
		return nil, err
	}
	if DeviceIDFromPublicKey(pub) != id.DeviceID {
		return nil, errors.New("device_id does not match public key")
	}
	return &id, nil
}

// CanonicalAuthMessage builds the bytes a device signs for session authentication.
func CanonicalAuthMessage(deviceID string, timestamp int64) []byte {
	return []byte(fmt.Sprintf("mynet-auth-v1\n%s\n%d\n", deviceID, timestamp))
}

// CanonicalEnrollMessage builds the bytes signed during enrollment.
func CanonicalEnrollMessage(deviceID, name string, timestamp int64) []byte {
	return []byte(fmt.Sprintf("mynet-enroll-v1\n%s\n%s\n%d\n", deviceID, name, timestamp))
}

// CanonicalVPNRegisterMessage binds a WireGuard public key to a device identity.
func CanonicalVPNRegisterMessage(deviceID, wgPublicKeyHex string, timestamp int64) []byte {
	return []byte(fmt.Sprintf("mynet-vpn-register-v1\n%s\n%s\n%d\n", deviceID, wgPublicKeyHex, timestamp))
}
