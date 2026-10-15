package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteDevicePersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "devices.db")
	store := NewStore("testnet", "hash")
	store.dbPath = path
	rec := &DeviceRecord{
		DeviceID:  "abc",
		Name:      "alpha",
		PublicKey: "pk",
		NetworkID: "testnet",
		Enrolled:  time.Now().UTC(),
	}
	if err := store.EnrollDevice(rec); err != nil {
		t.Fatal(err)
	}
	store2 := NewStore("testnet", "hash")
	if err := LoadDevicesFromSQLite(path, store2); err != nil {
		t.Fatal(err)
	}
	d, ok := store2.GetDevice("abc")
	if !ok || d.Name != "alpha" {
		t.Fatalf("reload failed %#v", d)
	}
	_ = os.Remove(path)
}
