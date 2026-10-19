package invitation

import (
	"encoding/base64"
	"testing"
)

func TestParseCompactRoundTrip(t *testing.T) {
	AllowHTTPForTest = true
	inv := Invite{
		Version:                Version,
		ServerURL:              "https://coordinator.example",
		NetworkID:              "default",
		CoordinatorFingerprint: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		EnrollmentToken:        "secret-token",
	}
	compact, err := inv.EncodeCompact()
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse(compact)
	if err != nil {
		t.Fatal(err)
	}
	if got.EnrollmentToken != inv.EnrollmentToken {
		t.Fatal("token mismatch")
	}
}

func TestRejectHTTPByDefault(t *testing.T) {
	AllowHTTPForTest = false
	inv := Invite{
		Version:                Version,
		ServerURL:              "http://insecure",
		CoordinatorFingerprint: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		EnrollmentToken:        "t",
	}
	if err := inv.Validate(); err == nil {
		t.Fatal("expected https required")
	}
}

func TestParseJSON(t *testing.T) {
	AllowHTTPForTest = true
	raw := `{"v":1,"server_url":"https://x","network_id":"n","coordinator_fingerprint":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","enrollment_token":"tok"}`
	inv, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if inv.NetworkID != "n" {
		t.Fatal(inv.NetworkID)
	}
	_ = base64.RawURLEncoding
}
