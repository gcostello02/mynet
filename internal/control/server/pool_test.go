package server

import (
	"net"
	"testing"
)

func TestNextFreeHost17231Pool(t *testing.T) {
	store := NewStore("test", "hash")
	_, pool, err := net.ParseCIDR("172.31.240.0/20")
	if err != nil {
		t.Fatal(err)
	}
	ip, err := store.nextFreeHost(pool)
	if err != nil {
		t.Fatal(err)
	}
	if ip != "172.31.240.1" {
		t.Fatalf("expected .1 got %s", ip)
	}
}

func TestAssignVPNIPv4DuplicateRejected(t *testing.T) {
	store := NewStore("test", "hash")
	_, pool, _ := net.ParseCIDR("172.31.240.0/20")
	d1 := &DeviceRecord{DeviceID: "a", Name: "mac", PublicKey: "pk1", NetworkID: "test", VPNIPv4: "172.31.240.1"}
	d2 := &DeviceRecord{DeviceID: "b", Name: "pc", PublicKey: "pk2", NetworkID: "test"}
	store.devices[d1.DeviceID] = d1
	store.devices[d2.DeviceID] = d2
	if err := store.AssignVPNIPv4("b", "172.31.240.1", pool); err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestAllocateRemapsOutsidePool(t *testing.T) {
	store := NewStore("test", "hash")
	_, pool, _ := net.ParseCIDR("172.31.240.0/20")
	d1 := &DeviceRecord{DeviceID: "a", Name: "mac", PublicKey: "pk1", NetworkID: "test", VPNIPv4: "10.77.0.1", WGPublicKey: repeatHex(64)}
	store.devices[d1.DeviceID] = d1
	ip, err := store.AllocateVPNIPv4(d1.DeviceID, d1.WGPublicKey, pool)
	if err != nil {
		t.Fatal(err)
	}
	if ip == "10.77.0.1" {
		t.Fatal("expected remap from old pool")
	}
	if !IPInPool(net.ParseIP(ip), pool) {
		t.Fatalf("%s outside pool", ip)
	}
}
