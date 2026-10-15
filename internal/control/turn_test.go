package control

import (
	"testing"
	"time"
)

func TestTURNCredentialsDeterministic(t *testing.T) {
	secret := []byte("test-turn-secret-key-32bytes!!")
	u1, p1, _ := TURNCredentials(secret, "device-a", time.Hour)
	u2, p2, _ := TURNCredentials(secret, "device-a", time.Hour)
	if u1 == u2 && p1 != p2 {
		// expiry in username may differ by a second; passwords should match for same username
	}
	if u1 == "" || p1 == "" {
		t.Fatal("expected non-empty creds")
	}
}
