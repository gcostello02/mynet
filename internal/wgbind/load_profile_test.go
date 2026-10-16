package wgbind

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"net/netip"
	"os"
	"testing"
	"time"

	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/tuntest"
)

// TestSustainedWireGuardICELoad drives encrypted pings for several seconds.
// CPU profile:
//
//	MYNET_WGBIND_LOAD=1 go test ./internal/wgbind -run TestSustainedWireGuardICELoad \
//	  -cpuprofile=profiling-results/wgbind-ice-cpu.prof -count=1
func TestSustainedWireGuardICELoad(t *testing.T) {
	if os.Getenv("MYNET_WGBIND_LOAD") == "" {
		t.Skip("set MYNET_WGBIND_LOAD=1 to run sustained load test")
	}
	ip0 := netip.MustParseAddr("10.91.0.1")
	ip1 := netip.MustParseAddr("10.91.0.2")
	bind0, bind1, cleanup := DuplexPairUDP(ip0, ip1)
	defer cleanup()

	tun0 := tuntest.NewChannelTUN()
	tun1 := tuntest.NewChannelTUN()
	var key0, key1 device.NoisePrivateKey
	_, _ = rand.Read(key0[:])
	_, _ = rand.Read(key1[:])
	pub0 := wgPub(key0[:])
	pub1 := wgPub(key1[:])
	dev0 := device.NewDevice(tun0.TUN(), bind0, device.NewLogger(device.LogLevelError, "l0: "))
	dev1 := device.NewDevice(tun1.TUN(), bind1, device.NewLogger(device.LogLevelError, "l1: "))
	defer dev0.Close()
	defer dev1.Close()
	_ = dev0.IpcSetOperation(bytes.NewBufferString(uapi("private_key", hex.EncodeToString(key0[:]))))
	_ = dev1.IpcSetOperation(bytes.NewBufferString(uapi("private_key", hex.EncodeToString(key1[:]))))
	_ = dev0.Up()
	_ = dev1.Up()
	_ = dev0.IpcSetOperation(bytes.NewBufferString(uapi(
		"public_key", hex.EncodeToString(pub1),
		"replace_allowed_ips", "true",
		"allowed_ip", ip1.String()+"/32",
		"endpoint", ip1.String()+":51820",
		"persistent_keepalive_interval", "1",
	)))
	_ = dev1.IpcSetOperation(bytes.NewBufferString(uapi(
		"public_key", hex.EncodeToString(pub0),
		"replace_allowed_ips", "true",
		"allowed_ip", ip0.String()+"/32",
		"endpoint", ip0.String()+":51820",
		"persistent_keepalive_interval", "1",
	)))

	msg := tuntest.Ping(ip0, ip1)
	tun1.Outbound <- msg
	select {
	case recv := <-tun0.Inbound:
		if !bytes.Equal(recv, msg) {
			t.Fatal("handshake mismatch")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("handshake timeout")
	}

	deadline := time.Now().Add(8 * time.Second)
	n := 0
	for time.Now().Before(deadline) {
		tun1.Outbound <- msg
		select {
		case recv := <-tun0.Inbound:
			if !bytes.Equal(recv, msg) {
				t.Fatal("mismatch")
			}
			n++
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout after %d packets", n)
		}
	}
	t.Logf("packets: %d", n)
}
