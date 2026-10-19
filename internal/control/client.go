package control

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/grantcostello/mynet/internal/identity"
)

// APIClient talks to the coordination server.
type APIClient struct {
	BaseURL    string
	HTTPClient *http.Client
	Token      string
}

func NewAPIClient(baseURL string) *APIClient {
	return &APIClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *APIClient) Enroll(id *identity.DeviceIdentity, enrollmentSecret string) error {
	ts := time.Now().UTC().Unix()
	msg := identity.CanonicalEnrollMessage(id.DeviceID, id.Name, ts)
	sig, err := id.Sign(msg)
	if err != nil {
		return err
	}
	body := EnrollRequest{
		DeviceID:         id.DeviceID,
		Name:             id.Name,
		PublicKey:        id.PublicKey,
		Timestamp:        ts,
		Signature:        base64.StdEncoding.EncodeToString(sig),
		EnrollmentSecret: enrollmentSecret,
	}
	return c.postJSON("/v1/enroll", body, nil, false)
}

func (c *APIClient) Auth(id *identity.DeviceIdentity) (AuthResponse, error) {
	ts := time.Now().UTC().Unix()
	msg := identity.CanonicalAuthMessage(id.DeviceID, ts)
	sig, err := id.Sign(msg)
	if err != nil {
		return AuthResponse{}, err
	}
	body := AuthRequest{
		DeviceID:  id.DeviceID,
		Timestamp: ts,
		Signature: base64.StdEncoding.EncodeToString(sig),
	}
	var resp AuthResponse
	if err := c.postJSON("/v1/auth", body, &resp, false); err != nil {
		return AuthResponse{}, err
	}
	c.Token = resp.Token
	return resp, nil
}

func (c *APIClient) ICEServers() (ICEServersResponse, error) {
	var resp ICEServersResponse
	if err := c.getJSON("/v1/ice-servers", &resp); err != nil {
		return ICEServersResponse{}, err
	}
	return resp, nil
}

func (c *APIClient) Peers() ([]PeerInfo, error) {
	var peers []PeerInfo
	if err := c.getJSON("/v1/peers", &peers); err != nil {
		return nil, err
	}
	return peers, nil
}

func (c *APIClient) VPNRegister(id *identity.DeviceIdentity, wgPublicKeyHex string) (VPNRegisterResponse, error) {
	ts := time.Now().UTC().Unix()
	msg := identity.CanonicalVPNRegisterMessage(id.DeviceID, wgPublicKeyHex, ts)
	sig, err := id.Sign(msg)
	if err != nil {
		return VPNRegisterResponse{}, err
	}
	body := VPNRegisterRequest{
		WGPublicKey: wgPublicKeyHex,
		Timestamp:   ts,
		Signature:   base64.StdEncoding.EncodeToString(sig),
	}
	var resp VPNRegisterResponse
	if err := c.postJSON("/v1/vpn/register", body, &resp, true); err != nil {
		return VPNRegisterResponse{}, err
	}
	return resp, nil
}

func (c *APIClient) VPNConfig() (VPNConfigResponse, error) {
	var resp VPNConfigResponse
	if err := c.getJSON("/v1/vpn/config", &resp); err != nil {
		return VPNConfigResponse{}, WrapVPNConfigError(err)
	}
	return resp, nil
}

// WrapVPNConfigError turns API failures into actionable CLI messages.
func WrapVPNConfigError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "http 404") {
		return fmt.Errorf(
			"coordination server has no VPN API (deploy mynet-server built from V0.2+); unauthenticated probe should return 401, not 404: %w",
			err,
		)
	}
	if strings.Contains(msg, "http 412") || strings.Contains(msg, "vpn not registered") {
		return fmt.Errorf("VPN not registered on the coordination server — run: mynet vpn register (%w)", err)
	}
	return err
}

func (c *APIClient) CreateSession(peerName string) (CreateSessionResponse, error) {
	body := CreateSessionRequest{PeerName: peerName}
	var resp CreateSessionResponse
	if err := c.postJSON("/v1/sessions", body, &resp, true); err != nil {
		return CreateSessionResponse{}, err
	}
	return resp, nil
}

func (c *APIClient) ConnectWebSocket() (*websocket.Conn, error) {
	u, err := c.webSocketURL()
	if err != nil {
		return nil, err
	}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	return conn, err
}

func (c *APIClient) webSocketURL() (*url.URL, error) {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	default:
		return nil, fmt.Errorf("unsupported server url scheme %q", u.Scheme)
	}
	u.Path = "/v1/ws"
	q := u.Query()
	q.Set("token", c.Token)
	u.RawQuery = q.Encode()
	return u, nil
}

func (c *APIClient) postJSON(path string, body any, out any, auth bool) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if auth && c.Token != "" {
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
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func (c *APIClient) getJSON(path string, out any) error {
	req, err := http.NewRequest(http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
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
	return json.NewDecoder(resp.Body).Decode(out)
}

func decodeAPIError(resp *http.Response) error {
	var buf struct {
		Error string `json:"error"`
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	_ = json.Unmarshal(data, &buf)
	msg := buf.Error
	if msg == "" {
		msg = "request failed"
	}
	return &APIError{Status: resp.StatusCode, Message: msg}
}
