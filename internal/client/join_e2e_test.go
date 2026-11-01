package client

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gcostello02/mynet/internal/control"
	"github.com/gcostello02/mynet/internal/control/server"
	"github.com/gcostello02/mynet/internal/invitation"
)

func TestJoinThreeDevices(t *testing.T) {
	invitation.AllowHTTPForTest = true
	control.AllowInsecureHTTPForTest = true
	defer func() {
		invitation.AllowHTTPForTest = false
		control.AllowInsecureHTTPForTest = false
	}()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "coord.db")
	srv, err := server.New(server.Config{
		ListenAddr:   ":0",
		ServerSecret: bytes.Repeat([]byte("x"), 32),
		DataPath:     dbPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	setupReq := httptest.NewRequest(http.MethodPost, "/v1/admin/setup", bytes.NewReader([]byte(
		`{"username":"admin","password":"longpassword12","network_display_name":"lab","public_server_url":"http://test"}`)))
	setupReq.RemoteAddr = "127.0.0.1:1"
	setupRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(setupRec, setupReq)
	if setupRec.Code != http.StatusCreated {
		t.Fatalf("setup %d", setupRec.Code)
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	meta := srv.Store().Meta().Snapshot()
	fp := meta.CoordinatorFingerprint
	coordPub := meta.CoordinatorPublicKeyB64

	for i := 0; i < 3; i++ {
		_, token, err := srv.Store().CreateInvitation(fmt.Sprintf("d%d", i), 3600, 1, "admin")
		if err != nil {
			t.Fatal(err)
		}
		inv := invitation.Invite{
			Version:                invitation.Version,
			ServerURL:              ts.URL,
			NetworkID:              "default",
			CoordinatorFingerprint: fp,
			CoordinatorPublicKey:   coordPub,
			EnrollmentToken:        token,
		}
		sd := filepath.Join(dir, fmt.Sprintf("client%d", i))
		rt := NewRuntime(sd, nil)
		if err := rt.Join(inv, JoinOptions{DeviceName: fmt.Sprintf("host%d", i)}); err != nil {
			t.Fatalf("join %d: %v", i, err)
		}
	}
}
