package wgbind

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/netip"
	"testing"
	"time"

	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/tuntest"
)

// Simulates the pre-V0.2.3 daemon waitPeerConn that read from the shared ICE conn.
func stealOneByteReads(c net.Conn, stop <-chan struct{}) {
	buf := make([]byte, 1)
	for {
		select {
		case <-stop:
			return
		default:
		}
		_ = c.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		if _, err := c.Read(buf); err == nil {
			continue
		}
	}
}

func TestCompetingReaderBreaksWireGuardOverICEBind(t *testing.T) {
	ip0 := netip.MustParseAddr("10.77.0.1")
	ip1 := netip.MustParseAddr("10.77.0.2")
	a, b := net.Pipe()
	bind0 := NewICEBind()
	bind1 := NewICEBind()
	bind0.SetPeerConn(ip1, a)
	bind1.SetPeerConn(ip0, b)

	stop := make(chan struct{})
	go stealOneByteReads(a, stop)
	go stealOneByteReads(b, stop)
	defer close(stop)

	if !encryptedPingOverBinds(t, bind0, bind1, ip0, ip1, 2*time.Second) {
		t.Log("expected failure when ICE conn has competing readers")
	}
}

func TestWireGuardOverICEBindWithoutCompetingReader(t *testing.T) {
	ip0 := netip.MustParseAddr("10.77.0.10")
	ip1 := netip.MustParseAddr("10.77.0.11")
	bind0, bind1, cleanup := DuplexPairUDP(ip0, ip1)
	defer cleanup()
	if !encryptedPingOverBinds(t, bind0, bind1, ip0, ip1, 10*time.Second) {
		t.Fatal("handshake/ping should succeed with single reader per ICE conn")
	}
}

func encryptedPingOverBinds(t *testing.T, bind0, bind1 *ICEBind, ip0, ip1 netip.Addr, timeout time.Duration) bool {
	tun0 := tuntest.NewChannelTUN()
	tun1 := tuntest.NewChannelTUN()

	var key0, key1 device.NoisePrivateKey
	_, _ = rand.Read(key0[:])
	_, _ = rand.Read(key1[:])
	pub0 := wgPub(key0[:])
	pub1 := wgPub(key1[:])

	dev0 := device.NewDevice(tun0.TUN(), bind0, device.NewLogger(device.LogLevelError, "t0: "))
	dev1 := device.NewDevice(tun1.TUN(), bind1, device.NewLogger(device.LogLevelError, "t1: "))
	defer dev0.Close()
	defer dev1.Close()

	_ = dev0.IpcSetOperation(bytes.NewBufferString(uapi("private_key", hex.EncodeToString(key0[:]))))
	_ = dev1.IpcSetOperation(bytes.NewBufferString(uapi("private_key", hex.EncodeToString(key1[:]))))
	if err := dev0.Up(); err != nil {
		t.Fatal(err)
	}
	if err := dev1.Up(); err != nil {
		t.Fatal(err)
	}
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
		return bytes.Equal(msg, recv)
	case <-time.After(timeout):
		return false
	}
}
