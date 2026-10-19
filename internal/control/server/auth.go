package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const sessionTokenTTL = 24 * time.Hour

type sessionClaims struct {
	DeviceID  string `json:"device_id"`
	ExpiresAt int64  `json:"exp"`
	AuthEpoch int    `json:"auth_epoch"`
}

// issueSessionToken creates an HMAC-signed opaque bearer token.
func issueSessionToken(deviceID string, authEpoch int, serverSecret []byte) (string, time.Time, error) {
	exp := time.Now().UTC().Add(sessionTokenTTL)
	claims := sessionClaims{DeviceID: deviceID, ExpiresAt: exp.Unix(), AuthEpoch: authEpoch}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", time.Time{}, err
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, serverSecret)
	mac.Write([]byte(payloadB64))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payloadB64 + "." + sig, exp, nil
}

func decodeSessionClaims(token string, serverSecret []byte) (sessionClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return sessionClaims{}, errors.New("invalid token format")
	}
	payloadB64, sigB64 := parts[0], parts[1]
	mac := hmac.New(sha256.New, serverSecret)
	mac.Write([]byte(payloadB64))
	expected := mac.Sum(nil)
	sig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return sessionClaims{}, errors.New("invalid token signature encoding")
	}
	if !hmac.Equal(sig, expected) {
		return sessionClaims{}, errors.New("invalid token signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return sessionClaims{}, errors.New("invalid token payload encoding")
	}
	var claims sessionClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return sessionClaims{}, err
	}
	if time.Now().UTC().Unix() > claims.ExpiresAt {
		return sessionClaims{}, errors.New("token expired")
	}
	if claims.DeviceID == "" {
		return sessionClaims{}, errors.New("missing device id in token")
	}
	return claims, nil
}

// verifySessionToken validates token integrity, expiry, and auth epoch.
func verifySessionToken(token string, serverSecret []byte, currentEpoch int) (string, error) {
	claims, err := decodeSessionClaims(token, serverSecret)
	if err != nil {
		return "", err
	}
	if claims.AuthEpoch != currentEpoch {
		return "", errors.New("token revoked")
	}
	return claims.DeviceID, nil
}

func hashEnrollmentSecret(secret string) string {
	sum := sha256.Sum256([]byte("mynet-enrollment-v1\n" + secret))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func constantTimeStringEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

func constantTimeSecretMatch(gotHash, wantHash string) bool {
	if len(gotHash) != len(wantHash) {
		return false
	}
	var v byte
	for i := 0; i < len(gotHash); i++ {
		v |= gotHash[i] ^ wantHash[i]
	}
	return v == 0
}

func validateTimestamp(ts int64, skew time.Duration) error {
	now := time.Now().UTC().Unix()
	if ts < now-int64(skew.Seconds()) || ts > now+int64(skew.Seconds()) {
		return fmt.Errorf("timestamp outside allowed skew")
	}
	return nil
}
