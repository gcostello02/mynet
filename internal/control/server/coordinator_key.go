package server

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const coordinatorKeyFile = "coordinator.ed25519"

var (
	errCoordinatorKeyMissing   = errors.New("coordinator private key file missing")
	errCoordinatorKeyCorrupt   = errors.New("coordinator private key file corrupt")
	errCoordinatorKeyMismatch  = errors.New("coordinator private key does not match database public key")
)

type coordinatorKeyOnDisk struct {
	Version       int    `json:"version"`
	PrivateKeyB64 string `json:"private_key_b64"`
}

func coordinatorKeyPath(dataPath string) string {
	return filepath.Join(filepath.Dir(dataPath), coordinatorKeyFile)
}

func saveCoordinatorIdentity(dataPath string, id *CoordinatorIdentity) error {
	if dataPath == "" || id == nil || len(id.PrivateKey) == 0 {
		return fmt.Errorf("invalid coordinator identity")
	}
	dir := filepath.Dir(dataPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path := coordinatorKeyPath(dataPath)
	payload := coordinatorKeyOnDisk{
		Version:       1,
		PrivateKeyB64: base64.StdEncoding.EncodeToString(id.PrivateKey),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return err
	}
	return nil
}

func loadCoordinatorIdentity(dataPath string, expectPubB64, expectFingerprint string) (*CoordinatorIdentity, error) {
	path := coordinatorKeyPath(dataPath)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errCoordinatorKeyMissing
		}
		return nil, err
	}
	var disk coordinatorKeyOnDisk
	if err := json.Unmarshal(raw, &disk); err != nil || disk.Version != 1 {
		return nil, errCoordinatorKeyCorrupt
	}
	privRaw, err := base64.StdEncoding.DecodeString(disk.PrivateKeyB64)
	if err != nil || len(privRaw) != ed25519.PrivateKeySize {
		return nil, errCoordinatorKeyCorrupt
	}
	priv := ed25519.PrivateKey(privRaw)
	pub := priv.Public().(ed25519.PublicKey)
	id := &CoordinatorIdentity{
		PublicKey:   pub,
		PrivateKey:  priv,
		Fingerprint: fingerprintCoordinator(pub),
	}
	if expectPubB64 != "" && base64.StdEncoding.EncodeToString(pub) != expectPubB64 {
		return nil, errCoordinatorKeyMismatch
	}
	if expectFingerprint != "" && !constantTimeSecretMatch(id.Fingerprint, stringsLowerHex(expectFingerprint)) {
		return nil, errCoordinatorKeyMismatch
	}
	return id, nil
}

// CanonicalCoordinatorProofMessage builds the signed proof payload (v1, timestamp only).
func CanonicalCoordinatorProofMessage(networkID, fingerprint string, timestamp int64) []byte {
	return []byte(fmt.Sprintf("mynet-coordinator-proof-v1\n%s\n%s\n%d", networkID, fingerprint, timestamp))
}

// CanonicalCoordinatorProofMessageV2 binds a client challenge (replay resistance).
func CanonicalCoordinatorProofMessageV2(networkID, fingerprint, challengeB64 string, timestamp int64) []byte {
	return []byte(fmt.Sprintf("mynet-coordinator-proof-v2\n%s\n%s\n%s\n%d", networkID, fingerprint, challengeB64, timestamp))
}
