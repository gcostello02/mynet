package server

import (
	"net"
	"testing"
)

func TestAllocateVPNIPv4Stable(t *testing.T) {
	store := NewStore("test", "hash")
	_, pool, _ := net.ParseCIDR("10.77.0.0/16")
	d1 := &DeviceRecord{DeviceID: "a", Name: "macbook", PublicKey: "pk1", NetworkID: "test"}
	d2 := &DeviceRecord{DeviceID: "b", Name: "minipc", PublicKey: "pk2", NetworkID: "test"}
	store.devices[d1.DeviceID] = d1
	store.devices[d2.DeviceID] = d2
	store.byName[d1.Name] = d1.DeviceID
	store.byName[d2.Name] = d2.DeviceID

	k1 := repeatHex(64)
	k2 := repeatHex(64)
	k2 = "b" + k2[1:]
	ip1, err := store.AllocateVPNIPv4(d1.DeviceID, k1, pool)
	if err != nil {
		t.Fatal(err)
	}
	ip2, err := store.AllocateVPNIPv4(d2.DeviceID, k2, pool)
	if err != nil {
		t.Fatal(err)
	}
	if ip1 == ip2 {
		t.Fatal("duplicate ips")
	}
	ip1b, err := store.AllocateVPNIPv4(d1.DeviceID, k1, pool)
	if err != nil || ip1b != ip1 {
		t.Fatalf("expected stable %s got %s err %v", ip1, ip1b, err)
	}
}

func repeatHex(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'c'
	}
	return string(b)
}
