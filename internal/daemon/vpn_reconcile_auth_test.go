package daemon

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gcostello02/mynet/internal/client"
	"github.com/gcostello02/mynet/internal/control"
	"github.com/gcostello02/mynet/internal/control/server"
	"github.com/gcostello02/mynet/internal/invitation"
	"github.com/gcostello02/mynet/internal/tundev"
	"github.com/gcostello02/mynet/internal/wgbind"
	"github.com/gcostello02/mynet/internal/wgengine"
	"github.com/gcostello02/mynet/internal/wgkeys"
)

func TestReconcileVPNPeersRecoversExpiredSession(t *testing.T) {
	invitation.AllowHTTPForTest = true
	control.AllowInsecureHTTPForTest = true
	oldTTL := server.SessionTokenTTL
	server.SessionTokenTTL = 2 * time.Second
	defer func() {
		server.SessionTokenTTL = oldTTL
		invitation.AllowHTTPForTest = false
		control.AllowInsecureHTTPForTest = false
	}()

	dir := t.TempDir()
	rt, ts := enrollDaemonTestDevice(t, dir)
	defer ts.Close()

	tunMgr := tundev.NewManager()
	tunDev, _, err := tunMgr.Open("mynet0")
	if err != nil {
		t.Skip("TUN unavailable (need root/CAP_NET_ADMIN):", err)
	}
	defer func() { _ = tunMgr.Cleanup() }()

	bind := wgbind.NewICEBind()
	engine, err := wgengine.New(tunDev, bind)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	wgDir := filepath.Join(dir, "wg")
	wg, err := wgkeys.LoadOrCreate(wgDir)
	if err != nil {
		t.Fatal(err)
	}
	priv, err := wg.PrivateKeyHex()
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Up(priv, 51820); err != nil {
		t.Fatal(err)
	}

	peerCtx, peerCancel := context.WithCancel(context.Background())
	defer peerCancel()

	n := &Node{
		StateDir:      wgDir,
		running:       true,
		rt:            rt,
		engine:        engine,
		bind:          bind,
		tunMgr:        tunMgr,
		vpnPeerKeys:   map[string]string{},
		peerWorkerCtx: peerCtx,
		peerWorkers:   make(map[string]context.CancelFunc),
		reconcileMu:   sync.Mutex{},
	}

	time.Sleep(3 * time.Second)
	_, err = rt.APIClient().VPNConfig()
	if err == nil {
		t.Fatal("expected expired bearer before reconcile")
	}

	n.reconcileVPNPeers()

	if _, err := rt.APIClient().VPNConfig(); err != nil {
		t.Fatalf("vpn config after reconcile recovery: %v", err)
	}
	if rt.AuthTerminal() {
		t.Fatal("expected non-terminal auth after recovery")
	}
}

func enrollDaemonTestDevice(t *testing.T, dir string) (*client.Runtime, *httptest.Server) {
	dbPath := filepath.Join(dir, "coord.db")
	srv, err := server.New(server.Config{
		ListenAddr:   ":0",
		ServerSecret: bytes.Repeat([]byte("x"), 32),
		DataPath:     dbPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	setup := httptest.NewRequest(http.MethodPost, "/v1/admin/setup", bytes.NewReader([]byte(
		`{"username":"admin","password":"longpassword12","network_display_name":"lab","public_server_url":"http://test"}`)))
	setup.RemoteAddr = "127.0.0.1:1"
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, setup)

	_, token, err := srv.Store().CreateInvitation("edge", 3600, 1, "admin")
	if err != nil {
		t.Fatal(err)
	}
	meta := srv.Store().Meta().Snapshot()
	inv := invitation.Invite{
		Version:                invitation.Version,
		ServerURL:              ts.URL,
		NetworkID:              "default",
		CoordinatorFingerprint: meta.CoordinatorFingerprint,
		CoordinatorPublicKey:   meta.CoordinatorPublicKeyB64,
		EnrollmentToken:        token,
	}
	sd := filepath.Join(dir, "device")
	rt := client.NewRuntime(sd, nil)
	if err := rt.Join(inv, client.JoinOptions{DeviceName: "minipc"}); err != nil {
		t.Fatal(err)
	}
	return rt, ts
}
