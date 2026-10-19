package control

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/grantcostello/mynet/internal/identity"
)

// NetworkPublicResponse is returned by GET /v1/network/public.
type NetworkPublicResponse struct {
	NetworkID              string `json:"network_id"`
	CoordinatorPublicKey   string `json:"coordinator_public_key"`
	CoordinatorFingerprint string `json:"coordinator_fingerprint"`
	PublicServerURL        string `json:"public_server_url"`
	NetworkDisplayName     string `json:"network_display_name"`
}

// NetworkProofResponse is returned by POST /v1/network/proof.
type NetworkProofResponse struct {
	NetworkID              string `json:"network_id"`
	CoordinatorFingerprint string `json:"coordinator_fingerprint"`
	CoordinatorPublicKey   string `json:"coordinator_public_key"`
	Timestamp              int64  `json:"timestamp"`
	Challenge              string `json:"challenge,omitempty"`
	Signature              string `json:"signature"`
	ProofVersion           int    `json:"proof_version"`
}

func (c *APIClient) NetworkPublic() (NetworkPublicResponse, error) {
	var resp NetworkPublicResponse
	if err := c.getJSON("/v1/network/public", &resp); err != nil {
		return NetworkPublicResponse{}, err
	}
	return resp, nil
}

func (c *APIClient) NetworkProof(timestamp int64, challengeB64 string) (NetworkProofResponse, error) {
	body := map[string]any{"timestamp": timestamp}
	if challengeB64 != "" {
		body["challenge"] = challengeB64
	}
	var resp NetworkProofResponse
	if err := c.postJSON("/v1/network/proof", body, &resp, false); err != nil {
		return NetworkProofResponse{}, err
	}
	return resp, nil
}

// EnrollWithInvitation enrolls using a one-time invitation token and fingerprint pin.
func (c *APIClient) EnrollWithInvitation(id *identity.DeviceIdentity, enrollmentToken, coordinatorFingerprint string) error {
	ts := time.Now().UTC().Unix()
	msg := identity.CanonicalEnrollMessage(id.DeviceID, id.Name, ts)
	sig, err := id.Sign(msg)
	if err != nil {
		return err
	}
	req := EnrollRequest{
		DeviceID:               id.DeviceID,
		Name:                   id.Name,
		PublicKey:              id.PublicKey,
		Timestamp:              ts,
		Signature:              base64.StdEncoding.EncodeToString(sig),
		EnrollmentToken:        enrollmentToken,
		CoordinatorFingerprint: coordinatorFingerprint,
	}
	return c.postJSON("/v1/enroll", req, nil, false)
}

// Admin API types and methods
type AdminLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AdminLoginResponse struct {
	AdminID      string `json:"admin_id"`
	ExpiresAt    string `json:"expires_at"`
	SessionToken string `json:"session_token"`
}

func (c *APIClient) AdminLogin(username, password string) (AdminLoginResponse, error) {
	var resp AdminLoginResponse
	if err := c.postJSON("/v1/admin/login", AdminLoginRequest{Username: username, Password: password}, &resp, false); err != nil {
		return AdminLoginResponse{}, err
	}
	if resp.SessionToken != "" {
		c.Token = resp.SessionToken
	}
	return resp, nil
}

func (c *APIClient) AdminStatus() (map[string]any, error) {
	var resp map[string]any
	if err := c.getJSON("/v1/admin/status", &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *APIClient) AdminListDevices() ([]map[string]any, error) {
	var resp []map[string]any
	if err := c.getJSON("/v1/admin/devices", &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *APIClient) AdminCreateInvitation(label string, expiresInSec, maxUses int) (map[string]string, error) {
	body := map[string]any{
		"label":               label,
		"expires_in_seconds":  expiresInSec,
		"max_uses":            maxUses,
	}
	var resp map[string]string
	if err := c.postJSON("/v1/admin/invitations", body, &resp, true); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *APIClient) AdminRevokeDevice(deviceID string) error {
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/v1/admin/devices/"+deviceID+"/revoke", bytes.NewReader([]byte("{}")))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return decodeAPIError(resp)
	}
	return nil
}
