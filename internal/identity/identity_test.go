package identity

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratePersistLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "identity.json")
	id, err := Generate("test-device")
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(path, id); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected 0600, got %o", info.Mode().Perm())
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DeviceID != id.DeviceID {
		t.Fatalf("device id mismatch")
	}
	pub, err := loaded.PublicKeyBytes()
	if err != nil {
		t.Fatal(err)
	}
	msg := CanonicalAuthMessage(loaded.DeviceID, 123)
	sig, err := loaded.Sign(msg)
	if err != nil {
		t.Fatal(err)
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), msg, sig) {
		t.Fatal("signature invalid")
	}
}
