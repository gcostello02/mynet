package invitation

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const Version = 1

// AllowHTTPForTest disables HTTPS requirement (unit tests only).
var AllowHTTPForTest bool

// Invite is an administrator-issued credential for joining a network (protect like a password).
type Invite struct {
	Version                 int    `json:"v"`
	ServerURL               string `json:"server_url"`
	NetworkID               string `json:"network_id"`
	CoordinatorFingerprint  string `json:"coordinator_fingerprint"`
	CoordinatorPublicKey    string `json:"coordinator_public_key,omitempty"`
	EnrollmentToken         string `json:"enrollment_token"`
}

func (inv *Invite) Validate() error {
	if inv.Version != Version {
		return fmt.Errorf("unsupported invitation version %d", inv.Version)
	}
	if inv.ServerURL == "" || inv.EnrollmentToken == "" {
		return errors.New("invitation missing server_url or enrollment_token")
	}
	u, err := url.Parse(inv.ServerURL)
	if err != nil {
		return errors.New("invalid server_url")
	}
	if u.Scheme != "https" {
		if !AllowHTTPForTest || u.Scheme != "http" {
			return errors.New("coordinator server_url must use https")
		}
	}
	if u.Host == "" {
		return errors.New("invalid server_url host")
	}
	if inv.CoordinatorFingerprint == "" {
		return errors.New("invitation missing coordinator_fingerprint")
	}
	if len(inv.CoordinatorFingerprint) != 64 {
		return errors.New("invalid coordinator_fingerprint length")
	}
	if inv.NetworkID == "" {
		inv.NetworkID = "default"
	}
	return nil
}

// Encode serializes the invitation for out-of-band delivery (JSON, single line).
func (inv *Invite) Encode() ([]byte, error) {
	inv.Version = Version
	return json.Marshal(inv)
}

// EncodeCompact returns base64url(JSON) suitable for `mynet join`.
func (inv *Invite) EncodeCompact() (string, error) {
	raw, err := inv.Encode()
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// Parse decodes JSON or compact base64url invitation strings.
func Parse(s string) (Invite, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Invite{}, errors.New("empty invitation")
	}
	if strings.HasPrefix(s, "mynet-invite:") {
		s = strings.TrimPrefix(s, "mynet-invite:")
	}
	var raw []byte
	if strings.HasPrefix(s, "{") {
		raw = []byte(s)
	} else {
		dec, err := base64.RawURLEncoding.DecodeString(s)
		if err != nil {
			dec, err = base64.URLEncoding.DecodeString(s)
			if err != nil {
				return Invite{}, errors.New("invitation is not valid JSON or base64url")
			}
		}
		raw = dec
	}
	var inv Invite
	if err := json.Unmarshal(raw, &inv); err != nil {
		if !strings.HasPrefix(s, "{") {
			return Invite{}, errors.New("invalid invitation: this looks like a raw enrollment token; use the full invitation from the dashboard (Copy invitation) or `mynet admin invitations create`")
		}
		return Invite{}, fmt.Errorf("invalid invitation JSON: %w", err)
	}
	if err := inv.Validate(); err != nil {
		return Invite{}, err
	}
	return inv, nil
}
