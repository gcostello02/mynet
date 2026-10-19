package server

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gcostello02/mynet/internal/control"
	"github.com/gcostello02/mynet/internal/identity"
)

func TestInvitationAtomicEnrollConcurrentDBConnections(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "concurrent.db")
	srv, err := New(Config{
		ListenAddr:   ":0",
		ServerSecret: bytes.Repeat([]byte("s"), 32),
		DataPath:     dbPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	fp := adminSetup(t, srv)
	_, token, err := srv.store.CreateInvitation("race", time.Hour, 1, "admin")
	if err != nil {
		t.Fatal(err)
	}

	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			db, err := sql.Open("sqlite", sqliteDSN(dbPath))
			if err != nil {
				return
			}
			defer db.Close()
			store := NewStore("default", "")
			store.sqlDB = db
			store.dbPath = dbPath
			store.meta = srv.store.meta
			if err := loadInvitations(db, store.invitations); err != nil {
				return
			}
			dev, err := identity.Generate("race-device")
			if err != nil {
				return
			}
			rec := &DeviceRecord{
				DeviceID:  dev.DeviceID,
				Name:      dev.Name,
				PublicKey: dev.PublicKey,
				NetworkID: "default",
				Enrolled:  time.Now().UTC(),
			}
			if err := store.EnrollDeviceWithInvitationAtomic(token, rec); err != nil {
				return
			}
			wins.Add(1)
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("expected exactly 1 atomic enroll, got %d", wins.Load())
	}

	db, _ := sql.Open("sqlite", sqliteDSN(dbPath))
	defer db.Close()
	var uses int
	_ = db.QueryRow(`SELECT uses FROM invitations LIMIT 1`).Scan(&uses)
	if uses != 1 {
		t.Fatalf("expected invitation uses=1, got %d", uses)
	}
	var deviceCount int
	_ = db.QueryRow(`SELECT COUNT(*) FROM devices WHERE revoked = 0`).Scan(&deviceCount)
	if deviceCount != 1 {
		t.Fatalf("expected 1 device row, got %d", deviceCount)
	}

	// Fingerprint still valid on restarted coordinator.
	srv2, err := New(Config{ListenAddr: ":0", ServerSecret: bytes.Repeat([]byte("s"), 32), DataPath: dbPath})
	if err != nil {
		t.Fatal(err)
	}
	if srv2.coordinatorIdentity.Fingerprint != fp {
		t.Fatal("fingerprint mismatch")
	}
}

func TestInvitationEnrollConflictDoesNotConsumeUse(t *testing.T) {
	srv, _ := testServerDB(t, "")
	fp := adminSetup(t, srv)
	_, token, _ := srv.store.CreateInvitation("once", time.Hour, 1, "admin")
	id1, _ := identity.Generate("dup")
	enrollWithToken(t, srv, id1, token, fp)

	_, token2, _ := srv.store.CreateInvitation("second", time.Hour, 1, "admin")
	id2, _ := identity.Generate("dup")
	rec := enrollWithTokenRaw(srv, id2, token2, fp)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected conflict, got %d", rec.Code)
	}
	if err := srv.store.CheckInvitation(token2); err != nil {
		t.Fatalf("invitation should remain valid: %v", err)
	}
	var uses int
	_ = srv.store.sqlDB.QueryRow(`SELECT uses FROM invitations WHERE id IN (SELECT id FROM invitations ORDER BY created_at DESC LIMIT 1)`).Scan(&uses)
	if uses != 0 {
		t.Fatalf("failed enroll should not consume use, uses=%d", uses)
	}
}

func TestEnrollViaHTTPUsesAtomicPath(t *testing.T) {
	srv, _ := testServerDB(t, "")
	fp := adminSetup(t, srv)
	_, token, _ := srv.store.CreateInvitation("http", time.Hour, 1, "admin")
	id, _ := identity.Generate("http-enroll")
	ts := time.Now().UTC().Unix()
	msg := identity.CanonicalEnrollMessage(id.DeviceID, id.Name, ts)
	sig, _ := id.Sign(msg)
	body, _ := json.Marshal(control.EnrollRequest{
		DeviceID: id.DeviceID, Name: id.Name, PublicKey: id.PublicKey,
		Timestamp: ts, Signature: base64.StdEncoding.EncodeToString(sig),
		EnrollmentToken: token, CoordinatorFingerprint: fp,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/enroll", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.handleEnroll(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("enroll %d %s", rec.Code, rec.Body.String())
	}
	var uses int
	_ = srv.store.sqlDB.QueryRow(`SELECT uses FROM invitations LIMIT 1`).Scan(&uses)
	if uses != 1 {
		t.Fatalf("expected uses=1 after success, got %d", uses)
	}
}
