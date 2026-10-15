package tundev

import (
	"fmt"
	"net/netip"
	"sync"

	"golang.zx2c4.com/wireguard/tun"
)

// Manager owns TUN lifecycle and per-peer /32 routes for one VPN session.
type Manager struct {
	mu         sync.Mutex
	ifName     string
	tun        tun.Device
	routes     []netip.Addr
	configured bool
}

func NewManager() *Manager {
	return &Manager{}
}

// Plan describes network changes without applying them.
type Plan struct {
	TUNName      string
	LocalPrefix  string
	PeerRoutes   []string
	ConflictWarn []string
}

// BuildPlan returns intended interface and routes for preflight.
func BuildPlan(local netip.Prefix, peers []netip.Addr) Plan {
	p := Plan{
		TUNName:     "mynet0 (OS may assign utunN on macOS)",
		LocalPrefix: local.String(),
	}
	for _, peer := range peers {
		p.PeerRoutes = append(p.PeerRoutes, peer.String()+"/32 via TUN")
		if err := RouteConflict(peer); err != nil {
			p.ConflictWarn = append(p.ConflictWarn, err.Error())
		}
	}
	return p
}

func (m *Manager) Open(name string) (tun.Device, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.tun != nil {
		return m.tun, m.ifName, nil
	}
	dev, ifName, err := Open(name)
	if err != nil {
		return nil, "", err
	}
	m.tun = dev
	m.ifName = ifName
	return dev, ifName, nil
}

func (m *Manager) Configure(prefix netip.Prefix) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ifName == "" {
		return fmt.Errorf("tun not open")
	}
	if err := Configure(m.ifName, prefix); err != nil {
		return err
	}
	m.configured = true
	return nil
}

func (m *Manager) AddPeerRoute(peer netip.Addr) error {
	m.mu.Lock()
	ifName := m.ifName
	m.mu.Unlock()
	if ifName == "" {
		return fmt.Errorf("tun not open")
	}
	if err := RouteConflict(peer); err != nil {
		return err
	}
	if err := addHostRoute(peer, ifName); err != nil {
		return err
	}
	m.mu.Lock()
	m.routes = append(m.routes, peer)
	m.mu.Unlock()
	return nil
}

func (m *Manager) Cleanup() error {
	m.mu.Lock()
	ifName := m.ifName
	routes := append([]netip.Addr(nil), m.routes...)
	t := m.tun
	m.tun = nil
	m.ifName = ""
	m.routes = nil
	m.configured = false
	m.mu.Unlock()

	for _, peer := range routes {
		_ = deleteHostRoute(peer, ifName)
	}
	if t != nil {
		_ = t.Close()
	}
	return nil
}

func (m *Manager) InterfaceName() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ifName
}

func deleteHostRoute(peer netip.Addr, ifName string) error {
	return cleanupPeerRoute(peer, ifName)
}

// ResetGlobalRoutes clears package-level route tracking (tests).
func ResetGlobalRoutes() {
	routeMu.Lock()
	installedRoutes = nil
	routeMu.Unlock()
}
