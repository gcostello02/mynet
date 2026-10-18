//go:build !race

package wgengine

import (
	"crypto/rand"
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/gcostello02/mynet/internal/wgbind"
	"golang.org/x/crypto/curve25519"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/tuntest"
)

func TestNudgeHandshakeConfiguredPeer(t *testing.T) {
	ip1 := netip.MustParseAddr("10.77.3.2")
	bind := wgbind.NewICEBind()
	defer bind.Shutdown()
	tun := tuntest.NewChannelTUN()
	var key device.NoisePrivateKey
	_, _ = rand.Read(key[:])
	var peerSk device.NoisePrivateKey
	_, _ = rand.Read(peerSk[:])
	pubPeer := hex.EncodeToString(noisePublic(peerSk[:]))

	eng, err := New(tun.TUN(), bind)
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.Up(hex.EncodeToString(key[:]), 51820); err != nil {
		t.Fatal(err)
	}
	if err := eng.SetPeer(pubPeer, ip1, ip1.String()+"/32"); err != nil {
		t.Fatal(err)
	}
	if err := eng.NudgeHandshake(pubPeer); err != nil {
		t.Fatal(err)
	}
}

func noisePublic(priv []byte) []byte {
	var sk, pk [32]byte
	copy(sk[:], priv)
	curve25519.ScalarBaseMult(&pk, &sk)
	return pk[:]
}
