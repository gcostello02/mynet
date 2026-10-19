package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gcostello02/mynet/internal/client"
	"github.com/gcostello02/mynet/internal/config"
	"github.com/gcostello02/mynet/internal/identity"
	"github.com/gcostello02/mynet/internal/tundev"
	"github.com/gcostello02/mynet/internal/wgbind"
	"github.com/gcostello02/mynet/internal/wgengine"
	"github.com/gcostello02/mynet/internal/wgkeys"
)

func (n *Node) configuredVPNIPv4() string {
	if n.StateDir == "" {
		return ""
	}
	wg, err := wgkeys.Load(n.StateDir)
	if err != nil || wg.VPNIPv4 == "" {
		return ""
	}
	return wg.VPNIPv4
}

// Node is the long-running VPN client.
type Node struct {
	StateDir string
	Logger   *slog.Logger

	mu              sync.Mutex
	running         bool
	rt              *client.Runtime
	engine          *wgengine.Engine
	bind            *wgbind.ICEBind
	tunMgr          *tundev.Manager
	tunName         string
	vpnIP           netip.Addr
	peers           map[string]PeerStatus
	vpnPeerKeys     map[string]string // peer name -> wg public key hex
	peerCancel      context.CancelFunc
	signalingCancel context.CancelFunc
	netWatch        *NetWatcher
}

type PeerStatus struct {
	Name         string
	VPNIP        string
	Transport    string
	State        string
	TURNUsed     bool
	LocalType    string
	RemoteType   string
	LocalAddr    string
	RemoteAddr   string
	ICESessionID     string
	ICEConnState        string
	RecoveryAttempts    int
	ICEFailedAttempts   int
	TransportRecoveries int
	RecoveryState       string
	WGPublicKey         string
}

// Preflight returns planned network changes without applying them.
func (n *Node) Preflight() (tundev.Plan, config.PreflightPaths, error) {
	stateDir := n.StateDir
	if stateDir == "" {
		stateDir, _ = config.ResolveStateDir()
	}
	paths, err := config.CollectPreflightPaths()
	if err != nil {
		return tundev.Plan{}, paths, err
	}
	if err := config.IdentityMustExist(stateDir); err != nil {
		return tundev.Plan{}, paths, err
	}
	rt := client.NewRuntime(stateDir, n.Logger)
	if err := rt.LoadSessionOnly(); err != nil {
		return tundev.Plan{}, paths, fmt.Errorf("not logged in: run mynet login first (%w)", err)
	}
	cfg, err := rt.APIClient().VPNConfig()
	if err != nil {
		return tundev.Plan{}, paths, err
	}
	local, err := netip.ParsePrefix(cfg.Address)
	if err != nil {
		return tundev.Plan{}, paths, err
	}
	peers := make([]netip.Addr, 0, len(cfg.Peers))
	for _, p := range cfg.Peers {
		peers = append(peers, netip.MustParseAddr(p.VPNIPv4))
	}
	return tundev.BuildPlan(local, peers), paths, nil
}

// Up starts VPN: TUN, WireGuard, signaling, ICE mesh.
func (n *Node) Up(ctx context.Context) error {
	n.mu.Lock()
	if n.running {
		n.mu.Unlock()
		return nil
	}
	existingRT := n.rt
	existingSigCancel := n.signalingCancel
	n.mu.Unlock()

	stateDir := n.StateDir
	if stateDir == "" {
		d, err := config.ResolveStateDir()
		if err != nil {
			return err
		}
		stateDir = d
	}
	if err := config.IdentityMustExist(stateDir); err != nil {
		return err
	}
	id, err := identity.Load(config.IdentityPath(stateDir))
	if err != nil {
		return err
	}
	wgID, err := wgkeys.Load(stateDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			wgID, err = wgkeys.LoadOrCreate(stateDir)
		}
		if err != nil {
			return fmt.Errorf("wireguard keys: %w", err)
		}
	}
	privHex, err := wgID.PrivateKeyHex()
	if err != nil {
		return err
	}

	rt := existingRT
	var sigCancel context.CancelFunc
	if rt == nil {
		rt = client.NewRuntime(stateDir, n.Logger)
		rt.SetVPNMode(true)
		if err := rt.Login(""); err != nil {
			return fmt.Errorf("login: %w", err)
		}
		sigCtx, cancel := context.WithCancel(context.Background())
		sigCancel = cancel
		n.mu.Lock()
		n.rt = rt
		n.signalingCancel = sigCancel
		n.mu.Unlock()
		rt.StartSignaling(sigCtx)
	} else {
		sigCancel = existingSigCancel
	}
	if err := rt.WaitForSignaling(ctx, 30*time.Second); err != nil {
		if sigCancel != nil && existingRT == nil {
			sigCancel()
		}
		return fmt.Errorf("signaling: %w", err)
	}
	if _, err := rt.RegisterVPN(); err != nil {
		return err
	}
	vpnCfg, err := rt.APIClient().VPNConfig()
	if err != nil {
		return fmt.Errorf("vpn config: %w", err)
	}
	if err := wgkeys.SaveAssignedVPNIPv4(stateDir, stringsTrimHost(vpnCfg.Address)); err != nil {
		return fmt.Errorf("sync vpn address: %w", err)
	}
	addr, err := netip.ParsePrefix(vpnCfg.Address)
	if err != nil {
		return err
	}

	tunMgr := tundev.NewManager()
	tunDev, name, err := tunMgr.Open("mynet0")
	if err != nil {
		return fmt.Errorf("create TUN (requires root/CAP_NET_ADMIN): %w", err)
	}

	bind := wgbind.NewICEBind()
	engine, err := wgengine.New(tunDev, bind)
	if err != nil {
		_ = tunMgr.Cleanup()
		return fmt.Errorf("wireguard engine: %w", err)
	}

	abort := func(step string, err error) error {
		_ = engine.Close()
		_ = tunMgr.Cleanup()
		return fmt.Errorf("%s: %w", step, err)
	}

	if err := tunMgr.Configure(addr); err != nil {
		return abort("configure TUN "+name, err)
	}
	if err := engine.Up(privHex, 51820); err != nil {
		return abort("wireguard up on "+name, err)
	}

	vpnPeerKeys := make(map[string]string, len(vpnCfg.Peers))
	for _, p := range vpnCfg.Peers {
		peerIP := netip.MustParseAddr(p.VPNIPv4)
		if err := tunMgr.AddPeerRoute(peerIP); err != nil {
			return abort(fmt.Sprintf("route to peer %s", p.Name), err)
		}
		if err := engine.SetPeer(p.WGPublicKey, peerIP, p.VPNIPv4+"/32"); err != nil {
			return abort(fmt.Sprintf("wireguard peer %s", p.Name), err)
		}
		vpnPeerKeys[p.Name] = p.WGPublicKey
	}

	n.mu.Lock()
	n.running = true
	n.vpnPeerKeys = vpnPeerKeys
	n.rt = rt
	n.engine = engine
	n.bind = bind
	n.tunMgr = tunMgr
	n.tunName = name
	n.vpnIP = addr.Addr()
	n.peers = make(map[string]PeerStatus)
	peerCtx, peerCancel := context.WithCancel(context.Background())
	n.peerCancel = peerCancel
	n.mu.Unlock()

	n.startPeerWorkers(peerCtx, rt, id.DeviceID, vpnCfg.Peers)
	n.startNetWatch(peerCtx)
	n.startVPNReconcile(peerCtx)
	return nil
}

func (n *Node) startNetWatch(ctx context.Context) {
	n.mu.Lock()
	if n.netWatch != nil {
		n.mu.Unlock()
		return
	}
	rt := n.rt
	w := NewNetWatcher(DefaultNetSnapshot, func() {
		if n.Logger != nil {
			n.Logger.Info("network change detected; invalidating ICE transports")
		}
		InvalidatePeerTransports(rt)
	})
	n.netWatch = w
	n.mu.Unlock()
	w.Start(ctx)
}

func (n *Node) stopNetWatch() {
	n.mu.Lock()
	w := n.netWatch
	n.netWatch = nil
	n.mu.Unlock()
	if w != nil {
		w.Stop()
	}
}

func stringsTrimHost(addr string) string {
	if i := strings.Index(addr, "/"); i >= 0 {
		return addr[:i]
	}
	return addr
}

func shouldInitiateICE(selfID, peerID string) bool {
	return selfID < peerID
}

// VPNActive reports whether the VPN data plane is up.
func (n *Node) VPNActive() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.running
}

// Down tears down VPN data plane only. Signaling and the daemon IPC server stay up.
func (n *Node) Down() error {
	n.mu.Lock()
	if !n.running {
		n.mu.Unlock()
		return nil
	}
	peerCancel := n.peerCancel
	engine := n.engine
	tunMgr := n.tunMgr
	tunName := n.tunName
	n.peerCancel = nil
	n.running = false
	n.engine = nil
	n.bind = nil
	n.tunMgr = nil
	n.tunName = ""
	n.vpnIP = netip.Addr{}
	n.peers = nil
	n.vpnPeerKeys = nil
	n.mu.Unlock()

	n.stopNetWatch()
	if peerCancel != nil {
		peerCancel()
	}
	if engine != nil {
		_ = engine.Close()
	}
	if tunMgr != nil {
		_ = tunMgr.Cleanup()
	}
	_ = tundev.CleanupRoutes(tunName)
	return nil
}

// Shutdown tears down VPN and signaling (daemon exit).
func (n *Node) Shutdown() error {
	_ = n.Down()
	n.mu.Lock()
	sigCancel := n.signalingCancel
	rt := n.rt
	n.signalingCancel = nil
	n.rt = nil
	n.mu.Unlock()
	if sigCancel != nil {
		sigCancel()
	}
	if rt != nil {
		_ = rt.Down()
	}
	return nil
}

// Status snapshot for CLI.
type Status struct {
	DeviceName         string
	VPNIP              string
	Interface          string
	Signaling          string
	SignalingReconnect int
	SignalingLastError string
	SignalingLastMsg   string
	WireGuard          string
	Peers              []PeerStatus
	DaemonPID          int
	DaemonUptime       string
	LastLifecycleError string
}

func (n *Node) Status() Status {
	n.mu.Lock()
	defer n.mu.Unlock()
	st := Status{
		DeviceName: "",
		Interface:  n.tunName,
		WireGuard:  "Inactive",
		Signaling:  "Disconnected",
	}
	if n.running {
		st.WireGuard = "Active"
		st.VPNIP = n.vpnIP.String()
	} else if ip := n.configuredVPNIPv4(); ip != "" {
		st.WireGuard = "Stopped"
		st.VPNIP = ip + " (configured, inactive)"
	} else {
		st.WireGuard = "Stopped"
		st.VPNIP = "—"
	}
	if n.rt != nil {
		s, _ := n.rt.Status()
		st.DeviceName = s.DeviceName
		st.Signaling = s.Signaling
		snap := n.rt.SignalingSnapshot()
		if snap.State == "Connecting" {
			st.Signaling = "Connecting"
		}
		if s.LastError != "" && st.Signaling != "Connected" && st.Signaling != "Connecting" {
			st.SignalingLastError = s.LastError
		}
		st.SignalingReconnect = snap.ReconnectAttempts
		st.SignalingLastMsg = snap.LastMessageType
	}
	for _, p := range n.peers {
		st.Peers = append(st.Peers, p)
	}
	return st
}
