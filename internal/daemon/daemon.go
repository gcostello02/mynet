package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"sync"
	"time"

	"github.com/gcostello02/mynet/internal/client"
	"github.com/gcostello02/mynet/internal/config"
	"github.com/gcostello02/mynet/internal/control"
	"github.com/gcostello02/mynet/internal/identity"
	"github.com/gcostello02/mynet/internal/tundev"
	"github.com/gcostello02/mynet/internal/wgbind"
	"github.com/gcostello02/mynet/internal/wgengine"
	"github.com/gcostello02/mynet/internal/wgkeys"
)

// Node is the long-running VPN client.
type Node struct {
	StateDir string
	Logger   *slog.Logger

	mu      sync.Mutex
	running bool
	rt      *client.Runtime
	engine  *wgengine.Engine
	bind    *wgbind.ICEBind
	tunMgr  *tundev.Manager
	tunName string
	vpnIP   netip.Addr
	peers   map[string]PeerStatus
}

type PeerStatus struct {
	Name        string
	VPNIP       string
	Transport   string
	State       string
	TURNUsed    bool
	WGPublicKey string
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

	rt := client.NewRuntime(stateDir, n.Logger)
	rt.SetVPNMode(true)
	if err := rt.Login(""); err != nil {
		return fmt.Errorf("login: %w", err)
	}
	if _, err := rt.RegisterVPN(); err != nil {
		return err
	}
	vpnCfg, err := rt.APIClient().VPNConfig()
	if err != nil {
		return fmt.Errorf("vpn config: %w", err)
	}
	addr, err := netip.ParsePrefix(vpnCfg.Address)
	if err != nil {
		return err
	}

	tunMgr := tundev.NewManager()
	tunDev, name, err := tunMgr.Open("mynet0")
	if err != nil {
		return fmt.Errorf("tun (requires root/CAP_NET_ADMIN): %w", err)
	}
	if err := tunMgr.Configure(addr); err != nil {
		_ = tunMgr.Cleanup()
		return err
	}

	bind := wgbind.NewICEBind()
	engine, err := wgengine.New(tunDev, bind)
	if err != nil {
		_ = tunMgr.Cleanup()
		return err
	}
	if err := engine.Up(privHex, 51820); err != nil {
		_ = engine.Close()
		_ = tunMgr.Cleanup()
		return err
	}

	n.mu.Lock()
	n.running = true
	n.rt = rt
	n.engine = engine
	n.bind = bind
	n.tunMgr = tunMgr
	n.tunName = name
	n.vpnIP = addr.Addr()
	n.peers = make(map[string]PeerStatus)
	n.mu.Unlock()

	var setupErr error
	for _, p := range vpnCfg.Peers {
		if err := n.ensurePeer(ctx, rt, id.DeviceID, p); err != nil {
			n.Logger.Warn("peer setup", "peer", p.Name, "err", err)
			setupErr = err
		}
	}
	if setupErr != nil && len(vpnCfg.Peers) == 1 {
		_ = n.Down()
		return setupErr
	}
	return nil
}

func (n *Node) ensurePeer(ctx context.Context, rt *client.Runtime, selfID string, p control.VPNPeerConfig) error {
	peerIP := netip.MustParseAddr(p.VPNIPv4)
	if err := n.tunMgr.AddPeerRoute(peerIP); err != nil {
		return err
	}
	if err := n.engine.SetPeer(p.WGPublicKey, peerIP, p.VPNIPv4+"/32"); err != nil {
		return err
	}
	var link *client.PeerLink
	var err error
	if shouldInitiateICE(selfID, p.DeviceID) {
		link, err = rt.ConnectPeerVPN(p.Name)
		if err != nil {
			return err
		}
	} else {
		deadline := time.Now().Add(60 * time.Second)
		for time.Now().Before(deadline) {
			if l, ok := rt.PeerLink(p.Name); ok {
				link = l
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
		if link == nil {
			return fmt.Errorf("timed out waiting for ICE from peer %s", p.Name)
		}
	}
	n.bind.SetPeerConn(peerIP, link.Conn)
	n.mu.Lock()
	n.peers[p.Name] = PeerStatus{
		Name:        p.Name,
		VPNIP:       p.VPNIPv4,
		Transport:   link.Path.Connection,
		State:       "Connected",
		TURNUsed:    link.Path.TURNUsed,
		WGPublicKey: p.WGPublicKey,
	}
	n.mu.Unlock()
	return nil
}

func shouldInitiateICE(selfID, peerID string) bool {
	return selfID < peerID
}

// Down tears down VPN resources.
func (n *Node) Down() error {
	n.mu.Lock()
	if !n.running {
		n.mu.Unlock()
		return nil
	}
	engine := n.engine
	rt := n.rt
	tunMgr := n.tunMgr
	n.running = false
	n.engine = nil
	n.rt = nil
	n.bind = nil
	n.tunMgr = nil
	n.peers = nil
	n.mu.Unlock()

	if engine != nil {
		_ = engine.Close()
	}
	if rt != nil {
		_ = rt.Down()
	}
	if tunMgr != nil {
		_ = tunMgr.Cleanup()
	}
	_ = tundev.CleanupRoutes(n.tunName)
	return nil
}

// Status snapshot for CLI.
type Status struct {
	DeviceName string
	VPNIP      string
	Interface  string
	Signaling  string
	WireGuard  string
	Peers      []PeerStatus
}

func (n *Node) Status() Status {
	n.mu.Lock()
	defer n.mu.Unlock()
	st := Status{
		DeviceName: "",
		VPNIP:      n.vpnIP.String(),
		Interface:  n.tunName,
		WireGuard:  "Inactive",
		Signaling:  "Disconnected",
	}
	if n.running {
		st.WireGuard = "Active"
	}
	if n.rt != nil {
		s, _ := n.rt.Status()
		st.DeviceName = s.DeviceName
		st.Signaling = s.Signaling
	}
	for _, p := range n.peers {
		st.Peers = append(st.Peers, p)
	}
	return st
}
