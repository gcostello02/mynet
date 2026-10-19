package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Simulates a V0.3 devices-only SQLite file before schema v2.
func TestMigrateV03DevicesDB(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v03.db")
	db, err := openCoordinatorDB(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO devices (device_id, name, public_key, network_id, revoked, enrolled_at, wg_public_key, vpn_ipv4)
		VALUES ('abc', 'mini', 'pk', 'default', 0, ?, '', '172.31.240.2')`, time.Now().UTC().Format(time.RFC3339Nano))
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore("default", hashEnrollmentSecret("migrate-secret"))
	store.dbPath = path
	if err := attachCoordinatorDB(store, path); err != nil {
		t.Fatal(err)
	}
	devs := store.ListAllDevices()
	if len(devs) != 1 || devs[0].DeviceID != "abc" {
		t.Fatalf("expected migrated device, got %#v", devs)
	}
	// Second open applies migrations idempotently.
	if err := attachCoordinatorDB(store, path); err != nil {
		t.Fatal(err)
	}
}

func TestEmptyDBDoesNotAutoEnableLegacy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "coordinator.db")
	srv, err := New(Config{
		ListenAddr:       ":0",
		ServerSecret:     []byte("12345678901234567890123456789012"),
		EnrollmentSecret: "test-enroll-secret",
		DataPath:         path,
	})
	if err != nil {
		t.Fatal(err)
	}
	if srv.store.meta.LegacyEnrollmentEnabled {
		t.Fatal("legacy should be disabled on empty DB")
	}
}

func TestV03LegacyAutoEnable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v03.db")
	db, err := openCoordinatorDB(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO devices (device_id, name, public_key, network_id, revoked, enrolled_at, wg_public_key, vpn_ipv4)
		VALUES ('dev1', 'node', 'cHVibGlj', 'default', 0, ?, '', '')`, time.Now().UTC().Format(time.RFC3339Nano))
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	srv, err := New(Config{
		ListenAddr:       ":0",
		ServerSecret:     []byte("12345678901234567890123456789012"),
		EnrollmentSecret: "test-enroll-secret",
		DataPath:         path,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !srv.store.meta.LegacyEnrollmentEnabled {
		t.Fatal("expected legacy enabled for migrated V0.3 DB with devices")
	}
}

func TestRepeatedMigrationFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "coordinator.db")
	for i := 0; i < 3; i++ {
		db, err := openCoordinatorDB(path)
		if err != nil {
			t.Fatal(err)
		}
		db.Close()
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("expected non-empty db")
	}
}
