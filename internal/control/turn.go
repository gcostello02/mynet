package control

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"time"
)

// ICEServer is a STUN/TURN entry (WebRTC-style) returned by the coordination server.
type ICEServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

// ICEServersResponse is returned by GET /v1/ice-servers.
type ICEServersResponse struct {
	ICEServers []ICEServer `json:"ice_servers"`
}

// TURNCredentials builds coturn-compatible time-limited username/password.
// username format: "<expiryUnix>:<userID>"
func TURNCredentials(sharedSecret []byte, userID string, ttl time.Duration) (username, password string, expiry time.Time) {
	expiry = time.Now().UTC().Add(ttl)
	username = fmt.Sprintf("%d:%s", expiry.Unix(), userID)
	mac := hmac.New(sha1.New, sharedSecret)
	mac.Write([]byte(username))
	password = base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return username, password, expiry
}
