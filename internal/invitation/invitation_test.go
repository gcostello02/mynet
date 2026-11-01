package invitation

import (
	"encoding/base64"
	"strings"
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

func TestParseRejectsRawEnrollmentToken(t *testing.T) {
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
	parsed, err := Parse(compact)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Parse(parsed.EnrollmentToken)
	if err == nil {
		t.Fatal("expected error for raw token")
	}
	if !strings.Contains(err.Error(), "raw enrollment token") {
		t.Fatalf("unexpected error: %v", err)
	}
	// Decoded compact without JSON shape should also fail clearly.
	_, err = Parse(compact[:20])
	if err == nil {
		t.Fatal("expected error for truncated compact")
	}
	_, err = Parse(compact)
	if err != nil {
		t.Fatalf("full compact should parse: %v", err)
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
