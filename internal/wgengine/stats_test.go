package wgengine

import (
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/gcostello02/mynet/internal/wgbind"
	"golang.zx2c4.com/wireguard/tun/tuntest"
)

func TestPeerStatsByPublicKey(t *testing.T) {
	ch := tuntest.NewChannelTUN()
	eng, err := New(ch.TUN(), wgbind.NewICEBind())
	if err != nil {
		t.Fatal(err)
	}
	var priv [32]byte
	_, _ = rand.Read(priv[:])
	if err := eng.Up(hex.EncodeToString(priv[:]), 51820); err != nil {
		t.Fatal(err)
	}
	peerPub := make([]byte, 32)
	_, _ = rand.Read(peerPub)
	if err := eng.SetPeer(hex.EncodeToString(peerPub), mustAddr("10.77.0.2"), "10.77.0.2/32"); err != nil {
		t.Fatal(err)
	}
	st, ok, err := eng.PeerStatsByPublicKey(hex.EncodeToString(peerPub))
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("peer not found")
	}
	if st.PublicKeyFingerprint == "" {
		t.Fatal("expected fingerprint")
	}
	if len(st.AllowedIPs) != 1 || st.AllowedIPs[0] != "10.77.0.2/32" {
		t.Fatalf("allowed IPs: %v", st.AllowedIPs)
	}
	_ = eng.Close()
}
