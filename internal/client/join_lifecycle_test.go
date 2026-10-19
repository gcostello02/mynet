package client

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/grantcostello/mynet/internal/identity"
	"github.com/grantcostello/mynet/internal/control"
	"github.com/grantcostello/mynet/internal/config"
	"github.com/grantcostello/mynet/internal/control/server"
	"github.com/grantcostello/mynet/internal/invitation"
)

func testCoord(t *testing.T) (*httptest.Server, *server.Server, string, string) {
	invitation.AllowHTTPForTest = true
	control.AllowInsecureHTTPForTest = true
	dir := t.TempDir()
	srv, err := server.New(server.Config{
		ListenAddr:   ":0",
		ServerSecret: bytes.Repeat([]byte("s"), 32),
		DataPath:     filepath.Join(dir, "db.sqlite"),
	})
	if err != nil {
		t.Fatal(err)
	}
	setupReq := httptest.NewRequest(http.MethodPost, "/v1/admin/setup", bytes.NewReader([]byte(
		`{"username":"admin","password":"longpassword12","network_display_name":"lab","public_server_url":"http://test"}`)))
	setupReq.RemoteAddr = "127.0.0.1:1"
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, setupReq)
	ts := httptest.NewServer(srv.Handler())
	meta := srv.Store().Meta().Snapshot()
	return ts, srv, meta.CoordinatorFingerprint, meta.CoordinatorPublicKeyB64
}

func TestJoinRejectsExistingIdentityWithoutForce(t *testing.T) {
	ts, srv, fp, pub := testCoord(t)
	defer ts.Close()
	_, token, _ := srv.Store().CreateInvitation("a", 3600, 1, "admin")
	inv := invitation.Invite{
		Version: invitation.Version, ServerURL: ts.URL, NetworkID: "default",
		CoordinatorFingerprint: fp, CoordinatorPublicKey: pub, EnrollmentToken: token,
	}
	sd := filepath.Join(t.TempDir(), "c1")
	rt := NewRuntime(sd, nil)
	if err := rt.Join(inv, JoinOptions{DeviceName: "host"}); err != nil {
		t.Fatal(err)
	}
	rt2 := NewRuntime(sd, nil)
	if err := rt2.Join(inv, JoinOptions{DeviceName: "host2"}); err == nil {
		t.Fatal("expected error without --force")
	}
}

func TestJoin409RequiresSuccessfulAuth(t *testing.T) {
	ts, srv, fp, pub := testCoord(t)
	defer ts.Close()
	_, token, _ := srv.Store().CreateInvitation("b", 3600, 1, "admin")
	inv := invitation.Invite{
		Version: invitation.Version, ServerURL: ts.URL, NetworkID: "default",
		CoordinatorFingerprint: fp, CoordinatorPublicKey: pub, EnrollmentToken: token,
	}
	sd1 := filepath.Join(t.TempDir(), "c1")
	rt1 := NewRuntime(sd1, nil)
	if err := rt1.Join(inv, JoinOptions{DeviceName: "dup"}); err != nil {
		t.Fatal(err)
	}
	_, token2, _ := srv.Store().CreateInvitation("b2", 3600, 1, "admin")
	inv2 := inv
	inv2.EnrollmentToken = token2
	sd2 := filepath.Join(t.TempDir(), "c2")
	rt2 := NewRuntime(sd2, nil)
	if err := rt2.Join(inv2, JoinOptions{DeviceName: "dup"}); err == nil {
		t.Fatal("expected conflict without auth for different device")
	}
}

func TestJoinForceReplacesIdentity(t *testing.T) {
	ts, srv, fp, pub := testCoord(t)
	defer ts.Close()
	_, token, _ := srv.Store().CreateInvitation("c", 3600, 1, "admin")
	inv := invitation.Invite{
		Version: invitation.Version, ServerURL: ts.URL, NetworkID: "default",
		CoordinatorFingerprint: fp, CoordinatorPublicKey: pub, EnrollmentToken: token,
	}
	sd := filepath.Join(t.TempDir(), "c1")
	rt := NewRuntime(sd, nil)
	if err := rt.Join(inv, JoinOptions{DeviceName: "first"}); err != nil {
		t.Fatal(err)
	}
	id1, _ := identity.Load(config.IdentityPath(sd))
	_, token2, _ := srv.Store().CreateInvitation("c2", 3600, 1, "admin")
	inv.EnrollmentToken = token2
	if err := rt.Join(inv, JoinOptions{DeviceName: "second", Force: true}); err != nil {
		t.Fatal(err)
	}
	id2, _ := identity.Load(config.IdentityPath(sd))
	if id1.DeviceID == id2.DeviceID {
		t.Fatal("expected new identity after force")
	}
}

func TestJoinRejectsHTTPCoordinatorURL(t *testing.T) {
	inv := invitation.Invite{
		Version: invitation.Version, ServerURL: "http://example.com", NetworkID: "default",
		CoordinatorFingerprint: fmt.Sprintf("%064x", bytes.Repeat([]byte{2}, 32)),
		EnrollmentToken: "x",
	}
	invitation.AllowHTTPForTest = false
	rt := NewRuntime(t.TempDir(), nil)
	if err := rt.Join(inv, JoinOptions{DeviceName: "x"}); err == nil {
		t.Fatal("expected https requirement")
	}
}
