package wgbind

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"net/netip"
	"testing"
	"time"

	"golang.org/x/crypto/curve25519"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/conn/bindtest"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/tuntest"
)

func TestICEBindDuplexSmoke(t *testing.T) {
	ip0 := netip.MustParseAddr("10.77.0.1")
	ip1 := netip.MustParseAddr("10.77.0.2")
	a, b, cleanup := DuplexPairUDP(ip0, ip1)
	defer cleanup()
	fnsB, _, err := b.Open(0)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = a.Open(0)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Send([][]byte{[]byte("wg-smoke")}, &PeerEndpoint{VPNIP: ip1}); err != nil {
		t.Fatal(err)
	}
	pkts := [][]byte{make([]byte, 1500)}
	sizes := []int{0}
	eps := make([]conn.Endpoint, 1)
	n, err := fnsB[0](pkts, sizes, eps)
	if err != nil || n != 1 || sizes[0] != 8 {
		t.Fatalf("recv n=%d size=%d err=%v", n, sizes[0], err)
	}
}

// TestWireGuardEncryptedPingOverTestTransport verifies a full WG handshake and
// ICMP ping using bindtest (baseline). ICEBind packet relay is covered by TestICEBindDuplexSmoke.
func TestWireGuardEncryptedPingOverTestTransport(t *testing.T) {
	ip0 := netip.MustParseAddr("10.77.0.1")
	ip1 := netip.MustParseAddr("10.77.0.2")
	binds := bindtest.NewChannelBinds()

	tun0 := tuntest.NewChannelTUN()
	tun1 := tuntest.NewChannelTUN()

	var key0, key1 device.NoisePrivateKey
	_, _ = rand.Read(key0[:])
	_, _ = rand.Read(key1[:])
	pub0 := wgPub(key0[:])
	pub1 := wgPub(key1[:])

	dev0 := device.NewDevice(tun0.TUN(), binds[0], device.NewLogger(device.LogLevelError, "wg0: "))
	dev1 := device.NewDevice(tun1.TUN(), binds[1], device.NewLogger(device.LogLevelError, "wg1: "))

	base0 := uapi("private_key", hex.EncodeToString(key0[:]))
	base1 := uapi("private_key", hex.EncodeToString(key1[:]))
	_ = dev0.IpcSetOperation(bytes.NewBufferString(base0))
	_ = dev1.IpcSetOperation(bytes.NewBufferString(base1))
	if err := dev0.Up(); err != nil {
		t.Fatal(err)
	}
	if err := dev1.Up(); err != nil {
		t.Fatal(err)
	}
	peer0 := uapi(
		"public_key", hex.EncodeToString(pub1),
		"replace_allowed_ips", "true",
		"allowed_ip", "10.77.0.2/32",
		"endpoint", "127.0.0.1:1",
		"persistent_keepalive_interval", "25",
	)
	peer1 := uapi(
		"public_key", hex.EncodeToString(pub0),
		"replace_allowed_ips", "true",
		"allowed_ip", "10.77.0.1/32",
		"endpoint", "127.0.0.1:2",
		"persistent_keepalive_interval", "25",
	)
	_ = dev0.IpcSetOperation(bytes.NewBufferString(peer0))
	_ = dev1.IpcSetOperation(bytes.NewBufferString(peer1))
	defer dev0.Close()
	defer dev1.Close()

	time.Sleep(500 * time.Millisecond)

	msg := tuntest.Ping(ip0, ip1)
	tun1.Outbound <- msg
	select {
	case recv := <-tun0.Inbound:
		if !bytes.Equal(msg, recv) {
			t.Fatal("encrypted ping mismatch")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for encrypted ping")
	}
}

// TestWireGuardEncryptedPingOverICEBind runs the same encrypted ping through ICEBind + UDP duplex.
func TestWireGuardEncryptedPingOverICEBind(t *testing.T) {
	ip0 := netip.MustParseAddr("10.77.0.1")
	ip1 := netip.MustParseAddr("10.77.0.2")
	bind0, bind1, cleanup := DuplexPairUDP(ip0, ip1)
	defer cleanup()

	tun0 := tuntest.NewChannelTUN()
	tun1 := tuntest.NewChannelTUN()

	var key0, key1 device.NoisePrivateKey
	_, _ = rand.Read(key0[:])
	_, _ = rand.Read(key1[:])
	pub0 := wgPub(key0[:])
	pub1 := wgPub(key1[:])

	dev0 := device.NewDevice(tun0.TUN(), bind0, device.NewLogger(device.LogLevelError, "ice0: "))
	dev1 := device.NewDevice(tun1.TUN(), bind1, device.NewLogger(device.LogLevelError, "ice1: "))

	base0 := uapi("private_key", hex.EncodeToString(key0[:]))
	base1 := uapi("private_key", hex.EncodeToString(key1[:]))
	_ = dev0.IpcSetOperation(bytes.NewBufferString(base0))
	_ = dev1.IpcSetOperation(bytes.NewBufferString(base1))
	if err := dev0.Up(); err != nil {
		t.Fatal(err)
	}
	if err := dev1.Up(); err != nil {
		t.Fatal(err)
	}
	peer0 := uapi(
		"public_key", hex.EncodeToString(pub1),
		"replace_allowed_ips", "true",
		"allowed_ip", "10.77.0.2/32",
		"endpoint", "10.77.0.2:51820",
		"persistent_keepalive_interval", "25",
	)
	peer1 := uapi(
		"public_key", hex.EncodeToString(pub0),
		"replace_allowed_ips", "true",
		"allowed_ip", "10.77.0.1/32",
		"endpoint", "10.77.0.1:51820",
		"persistent_keepalive_interval", "25",
	)
	_ = dev0.IpcSetOperation(bytes.NewBufferString(peer0))
	_ = dev1.IpcSetOperation(bytes.NewBufferString(peer1))
	defer dev0.Close()
	defer dev1.Close()

	time.Sleep(500 * time.Millisecond)

	msg := tuntest.Ping(ip0, ip1)
	tun1.Outbound <- msg
	select {
	case recv := <-tun0.Inbound:
		if !bytes.Equal(msg, recv) {
			t.Fatal("encrypted ping mismatch over ICEBind")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for encrypted ping over ICEBind")
	}
}

func uapi(cfg ...string) string {
	var b bytes.Buffer
	for i := 0; i < len(cfg); i += 2 {
		b.WriteString(cfg[i])
		b.WriteByte('=')
		b.WriteString(cfg[i+1])
		b.WriteByte('\n')
	}
	return b.String()
}

func wgPub(priv []byte) []byte {
	var sk, pk [32]byte
	copy(sk[:], priv)
	curve25519.ScalarBaseMult(&pk, &sk)
	return pk[:]
}
