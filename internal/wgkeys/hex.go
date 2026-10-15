package wgkeys

import (
	"encoding/base64"
	"fmt"
)

// PrivateKeyHex returns the WireGuard UAPI private_key format.
func (id *Identity) PrivateKeyHex() (string, error) {
	b, err := base64.StdEncoding.DecodeString(id.PrivateKey)
	if err != nil {
		return "", err
	}
	if len(b) != 32 {
		return "", fmt.Errorf("invalid private key length")
	}
	return fmt.Sprintf("%x", b), nil
}
