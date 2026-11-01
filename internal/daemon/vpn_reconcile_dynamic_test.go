package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gcostello02/mynet/internal/client"
	"github.com/gcostello02/mynet/internal/control"
	"github.com/gcostello02/mynet/internal/tundev"
	"github.com/gcostello02/mynet/internal/wgbind"
	"github.com/gcostello02/mynet/internal/wgengine"
	"github.com/gcostello02/mynet/internal/wgkeys"
)

func TestStartPeerWorkersForVPNPeersSpawnsWorker(t *testing.T) {
	peerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rt := client.NewRuntime(t.TempDir(), nil)
	rt.SetVPNMode(true)

	n := &Node{
		running:       true,
		rt:            rt,
		selfDeviceID:  "device-a",
		peerWorkerCtx: peerCtx,
		vpnPeerKeys:   map[string]string{"macbook": "2222222222222222222222222222222222222222222222222222222222222222"},
		peerWorkers:   make(map[string]context.CancelFunc),
	}
	n.startPeerWorkersForVPNPeers([]control.VPNPeerConfig{
		{Name: "macbook", DeviceID: "device-b", VPNIPv4: "172.31.240.2", WGPublicKey: "2222222222222222222222222222222222222222222222222222222222222222"},
	})

	n.mu.Lock()
	defer n.mu.Unlock()
	if n.peerWorkers["macbook"] == nil {
		t.Fatal("expected peer worker for macbook")
	}
}

// Regression: device A already on VPN must start ICE workers when device B appears in VPN config.
func TestReconcileStartsPeerWorkersWhenNewPeerEnrolls(t *testing.T) {
	const keyA = "1111111111111111111111111111111111111111111111111111111111111111"
	const keyB = "2222222222222222222222222222222222222222222222222222222222222222"

	var cfgGen atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/vpn/config" {
			http.NotFound(w, r)
			return
		}
		gen := cfgGen.Load()
		resp := control.VPNConfigResponse{
			NetworkID:   "default",
			Address:     "172.31.240.1/32",
			WGPublicKey: keyA,
			Peers:       nil,
		}
		if gen >= 1 {
			resp.Peers = []control.VPNPeerConfig{
				{
					DeviceID:    "device-b",
					Name:        "macbook",
					VPNIPv4:     "172.31.240.2",
					WGPublicKey: keyB,
					AllowedIPs:  []string{"172.31.240.2/32"},
				},
			}
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	rt := client.NewRuntime(t.TempDir(), nil)
	rt.SetVPNMode(true)
	api := control.NewAPIClient(srv.URL)
	api.Token = "test-token"
	rt.SetAPIClientForTest(api)

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

	stateDir := t.TempDir()
	wg, err := wgkeys.LoadOrCreate(stateDir)
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

	var hookMu sync.Mutex
	var hooked []string
	testHookVPNPeersAdded = func(added []control.VPNPeerConfig) {
		hookMu.Lock()
		for _, p := range added {
			hooked = append(hooked, p.Name)
		}
		hookMu.Unlock()
	}
	defer func() { testHookVPNPeersAdded = nil }()

	n := &Node{
		StateDir:      stateDir,
		running:       true,
		rt:            rt,
		engine:        engine,
		bind:          bind,
		tunMgr:        tunMgr,
		vpnPeerKeys:   map[string]string{},
		peerWorkerCtx: peerCtx,
		selfDeviceID:  "device-a",
		peerWorkers:   make(map[string]context.CancelFunc),
		reconcileMu:   sync.Mutex{},
	}

	n.reconcileVPNPeers()
	n.mu.Lock()
	if len(n.vpnPeerKeys) != 0 {
		t.Fatalf("expected no peers initially, got %v", n.vpnPeerKeys)
	}
	n.mu.Unlock()

	cfgGen.Store(1)

	deadline := time.Now().Add(3 * time.Second)
	for {
		n.reconcileVPNPeers()
		hookMu.Lock()
		ok := len(hooked) > 0 && hooked[0] == "macbook"
		hookMu.Unlock()
		n.mu.Lock()
		worker := n.peerWorkers["macbook"] != nil
		hasKey := n.vpnPeerKeys["macbook"] == keyB
		n.mu.Unlock()
		if ok && worker && hasKey {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("reconcile did not start peer worker for new peer (hook=%v workers=%d keys=%v)", hooked, len(n.peerWorkers), n.vpnPeerKeys)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
