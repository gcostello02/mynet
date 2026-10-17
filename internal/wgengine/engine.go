package wgengine

import (
	"bytes"
	"fmt"
	"net/netip"
	"strings"

	"github.com/grantcostello/mynet/internal/wgbind"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
)

const defaultMTU = 1420

// Engine wraps wireguard-go with an ICE bind.
type Engine struct {
	dev  *device.Device
	bind *wgbind.ICEBind
	tun  tun.Device
}

func New(tunDev tun.Device, bind *wgbind.ICEBind) (*Engine, error) {
	// Silent device logger: wireguard-go logs transient "no transport" errors while ICE
	// is still connecting; mynet surfaces persistent failures via diagnose/status.
	logger := device.NewLogger(device.LogLevelSilent, "")
	dev := device.NewDevice(tunDev, bind, logger)
	return &Engine{dev: dev, bind: bind, tun: tunDev}, nil
}

func (e *Engine) Bind() *wgbind.ICEBind { return e.bind }

func (e *Engine) Up(privateKeyHex string, listenPort uint16) error {
	var cfg bytes.Buffer
	fmt.Fprintf(&cfg, "private_key=%s\n", privateKeyHex)
	if listenPort == 0 {
		listenPort = 51820
	}
	fmt.Fprintf(&cfg, "listen_port=%d\n", listenPort)
	if err := e.dev.IpcSetOperation(strings.NewReader(cfg.String())); err != nil {
		return err
	}
	return e.dev.Up()
}

func (e *Engine) Down() error {
	return e.dev.Down()
}

func (e *Engine) Close() error {
	_ = e.Down()
	e.dev.Close() // closes the TUN fd owned by wireguard-go
	if e.bind != nil {
		_ = e.bind.Shutdown()
	}
	return nil
}

// SetPeer configures or updates a WireGuard peer (UAPI key is allowed_ip, not allowed_ips).
func (e *Engine) SetPeer(publicKeyHex string, vpnIP netip.Addr, allowedCIDRs ...string) error {
	if len(allowedCIDRs) == 0 {
		return fmt.Errorf("at least one allowed_ip CIDR required")
	}
	ep := fmt.Sprintf("%s:%d", vpnIP, 51820)
	var cfg bytes.Buffer
	fmt.Fprintf(&cfg, "public_key=%s\n", publicKeyHex)
	fmt.Fprintf(&cfg, "replace_allowed_ips=true\n")
	fmt.Fprintf(&cfg, "endpoint=%s\n", ep)
	for _, cidr := range allowedCIDRs {
		fmt.Fprintf(&cfg, "allowed_ip=%s\n", cidr)
	}
	fmt.Fprintf(&cfg, "persistent_keepalive_interval=25\n")
	return e.dev.IpcSetOperation(strings.NewReader(cfg.String()))
}

func (e *Engine) RemovePeer(publicKeyHex string) error {
	var cfg bytes.Buffer
	fmt.Fprintf(&cfg, "public_key=%s\n", publicKeyHex)
	fmt.Fprintf(&cfg, "remove=true\n")
	return e.dev.IpcSetOperation(strings.NewReader(cfg.String()))
}

// Device exposes the underlying wireguard device for tests.
func (e *Engine) Device() *device.Device { return e.dev }

// StdBind is unused but documents alternative for local tests.
type StdBind = conn.Bind
