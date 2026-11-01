package wgengine

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"
)

// PeerStats is parsed WireGuard UAPI state for one peer (no private keys).
type PeerStats struct {
	PublicKeyHex         string
	PublicKeyFingerprint string
	Endpoint             string
	AllowedIPs           []string
	LastHandshake        time.Time
	TxBytes              uint64
	RxBytes              uint64
}

// PeerStatsByPublicKey returns IPC stats for a configured peer public key (hex).
func (e *Engine) PeerStatsByPublicKey(publicKeyHex string) (PeerStats, bool, error) {
	peers, err := e.AllPeerStats()
	if err != nil {
		return PeerStats{}, false, err
	}
	want := strings.ToLower(strings.TrimSpace(publicKeyHex))
	for _, p := range peers {
		if strings.ToLower(p.PublicKeyHex) == want {
			return p, true, nil
		}
	}
	return PeerStats{}, false, nil
}

// AllPeerStats parses device IpcGetOperation output.
func (e *Engine) AllPeerStats() ([]PeerStats, error) {
	var buf bytes.Buffer
	if err := e.dev.IpcGetOperation(io.Writer(&buf)); err != nil {
		return nil, err
	}
	return parsePeerStats(buf.String())
}

func parsePeerStats(uapi string) ([]PeerStats, error) {
	var out []PeerStats
	var cur *PeerStats
	sc := bufio.NewScanner(strings.NewReader(uapi))
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "public_key":
			if cur != nil && cur.PublicKeyHex != "" {
				out = append(out, *cur)
			}
			cur = &PeerStats{PublicKeyHex: val}
			cur.PublicKeyFingerprint = fingerprintPubKey(val)
		case "endpoint":
			if cur != nil {
				cur.Endpoint = val
			}
		case "allowed_ip":
			if cur != nil {
				cur.AllowedIPs = append(cur.AllowedIPs, val)
			}
		case "last_handshake_time_sec":
			if cur != nil {
				sec, _ := parseUint64(val)
				cur.LastHandshake = time.Unix(int64(sec), 0)
			}
		case "last_handshake_time_nsec":
			if cur != nil {
				nsec, _ := parseUint64(val)
				cur.LastHandshake = time.Unix(cur.LastHandshake.Unix(), int64(nsec))
			}
		case "tx_bytes":
			if cur != nil {
				cur.TxBytes, _ = parseUint64(val)
			}
		case "rx_bytes":
			if cur != nil {
				cur.RxBytes, _ = parseUint64(val)
			}
		}
	}
	if cur != nil && cur.PublicKeyHex != "" {
		out = append(out, *cur)
	}
	return out, sc.Err()
}

func fingerprintPubKey(hexKey string) string {
	raw, err := hex.DecodeString(strings.TrimSpace(hexKey))
	if err != nil || len(raw) == 0 {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:8])
}

func parseUint64(s string) (uint64, error) {
	var n uint64
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
