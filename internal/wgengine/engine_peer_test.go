package wgengine

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/netip"
	"strings"
	"testing"

	"github.com/gcostello02/mynet/internal/wgbind"
	"golang.org/x/crypto/curve25519"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/tuntest"
)

func TestSetPeerAllowedIPUAPI(t *testing.T) {
	ch := tuntest.NewChannelTUN()
	eng, err := New(ch.TUN(), wgbind.NewICEBind())
	if err != nil {
		t.Fatal(err)
	}
	var priv device.NoisePrivateKey
	_, _ = rand.Read(priv[:])
	if err := eng.Up(hex.EncodeToString(priv[:]), 51820); err != nil {
		t.Fatal(err)
	}
	peerPub := make([]byte, 32)
	_, _ = rand.Read(peerPub)
	if err := eng.SetPeer(hex.EncodeToString(peerPub), mustAddr("10.77.0.2"), "10.77.0.2/32", "10.77.0.0/24"); err != nil {
		t.Fatalf("SetPeer: %v", err)
	}
	var buf bytes.Buffer
	if err := eng.Device().IpcGetOperation(io.Writer(&buf)); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "allowed_ips=") {
		t.Fatalf("must not use allowed_ips in UAPI: %s", out)
	}
	if !strings.Contains(out, "allowed_ip=10.77.0.2/32") || !strings.Contains(out, "allowed_ip=10.77.0.0/24") {
		t.Fatalf("missing allowed_ip lines: %s", out)
	}
	if !strings.Contains(out, "public_key="+hex.EncodeToString(peerPub)) {
		t.Fatalf("peer key missing: %s", out)
	}
	_ = eng.Close()
}

func wgPub(priv []byte) []byte {
	var sk, pk [32]byte
	copy(sk[:], priv)
	curve25519.ScalarBaseMult(&pk, &sk)
	return pk[:]
}

func mustAddr(s string) netip.Addr {
	a, err := netip.ParseAddr(s)
	if err != nil {
		panic(err)
	}
	return a
}
