package client

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/grantcostello/mynet/internal/control"
	icepkg "github.com/grantcostello/mynet/internal/ice"
)

// Diagnostics is a detailed connectivity report for the CLI.
type Diagnostics struct {
	DeviceName         string
	PeerName           string
	ServerURL          string
	IndependentMode    bool
	Signaling          string
	Authentication     string
	CoordinationOK     bool
	PeerOnline         bool
	ICEState           string
	Connection         string
	TURNUsed           bool
	LocalType          string
	RemoteType         string
	LocalAddr          string
	RemoteAddr         string
	RTT                time.Duration
	LastComm           string
	LocalCandidates    []icepkg.CandidateInfo
	RemoteCandidates   []icepkg.CandidateInfo
	TailscaleSignaling string // No | Yes | Unverified
	TailscaleDataPath  string // No | Yes | Unverified
	Warnings           []string
	LastError          string
}

// Diagnose gathers signaling and path diagnostics for a peer.
func (r *Runtime) Diagnose(peerName string) (Diagnostics, error) {
	if err := r.load(); err != nil {
		return Diagnostics{}, err
	}
	d := Diagnostics{
		DeviceName:      r.identity.Name,
		PeerName:        peerName,
		ServerURL:       r.cfg.ServerURL,
		IndependentMode: r.cfg.Network.IndependentMode,
	}
	if r.api.Token == "" {
		d.Authentication = "Not logged in"
		d.Signaling = "Disconnected"
		d.TailscaleSignaling = "Unverified"
		d.TailscaleDataPath = "Unverified"
		return d, nil
	}
	d.Authentication = "Valid (session token present)"
	r.refreshICEServers()

	// Refresh signaling state without leaving a stray connection if peers/connect will manage WS.
	if err := r.ensureWebSocket(); err != nil {
		d.Warnings = append(d.Warnings, "signaling websocket: "+err.Error())
	}

	r.mu.Lock()
	wsUp := r.ws != nil
	d.ICEState = r.icePath.ConnectionState
	if d.ICEState == "" {
		d.ICEState = "idle"
	}
	d.Connection = r.icePath.Connection
	d.TURNUsed = r.icePath.TURNUsed
	d.LocalType = r.icePath.LocalType
	d.RemoteType = r.icePath.RemoteType
	d.LocalAddr = r.icePath.LocalAddr
	d.RemoteAddr = r.icePath.RemoteAddr
	d.RTT = r.lastRTT
	d.LastError = r.lastErr
	d.LocalCandidates = append([]icepkg.CandidateInfo(nil), r.lastLocalCandidates...)
	d.RemoteCandidates = append([]icepkg.CandidateInfo(nil), r.lastRemoteCandidates...)
	if r.connState == StateConnected && !r.lastPingAt.IsZero() {
		d.LastComm = r.lastPingAt.UTC().Format(time.RFC3339)
	}
	r.mu.Unlock()

	if wsUp {
		d.Signaling = "Connected"
	} else {
		d.Signaling = "Disconnected"
	}

	d.Warnings = append(d.Warnings, r.independentWarnings()...)

	d.TailscaleSignaling, d.TailscaleDataPath = r.tailscaleDependency(d)
	d.Warnings = append(d.Warnings, tailscalePathWarnings(d)...)

	if peerName != "" {
		peers, err := r.api.Peers()
		if err != nil {
			d.Warnings = append(d.Warnings, "peer list: "+err.Error())
		} else {
			for _, p := range peers {
				if p.Name == peerName {
					d.PeerOnline = p.Online
					break
				}
			}
		}
	}

	if err := r.probeServerHealth(); err != nil {
		d.Warnings = append(d.Warnings, "coordination server: "+err.Error())
	} else {
		d.CoordinationOK = true
	}

	return d, nil
}

func (r *Runtime) tailscaleDependency(d Diagnostics) (signaling, dataPath string) {
	signaling = classifyTailscaleSignaling(d.ServerURL)
	dataPath = "Unverified"
	if d.ICEState == "idle" || d.ICEState == "" {
		return signaling, dataPath
	}
	if d.LocalAddr == "" && d.RemoteAddr == "" {
		return signaling, dataPath
	}
	if endpointUsesTailscaleCGNAT(d.LocalAddr) || endpointUsesTailscaleCGNAT(d.RemoteAddr) {
		return signaling, "Yes"
	}
	if d.TURNUsed {
		dataPath = "Unverified"
		return signaling, dataPath
	}
	if d.Connection != "" && strings.EqualFold(d.ICEState, "connected") {
		dataPath = "No"
	}
	return signaling, dataPath
}

func classifyTailscaleSignaling(serverURL string) string {
	host := serverHost(serverURL)
	if host == "" {
		return "Unverified"
	}
	if ip := net.ParseIP(host); ip != nil {
		if isTailscaleCGNATIP(ip) {
			return "Yes"
		}
		return "No"
	}
	// Public hostname (not a 100.x literal).
	return "No"
}

func endpointUsesTailscaleCGNAT(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	ip := net.ParseIP(host)
	return ip != nil && isTailscaleCGNATIP(ip)
}

func tailscalePathWarnings(d Diagnostics) []string {
	var w []string
	if d.IndependentMode && endpointUsesTailscaleCGNAT(d.LocalAddr) {
		w = append(w, "selected local endpoint is in 100.64.0.0/10 (Tailscale CGNAT)")
	}
	if d.IndependentMode && endpointUsesTailscaleCGNAT(d.RemoteAddr) {
		w = append(w, "selected remote endpoint is in 100.64.0.0/10 (Tailscale CGNAT)")
	}
	return w
}

func (r *Runtime) independentWarnings() []string {
	var w []string
	host := serverHost(r.cfg.ServerURL)
	if ip := net.ParseIP(host); ip != nil && isTailscaleCGNATIP(ip) && r.cfg.Network.IndependentMode {
		w = append(w, "server_url uses a Tailscale CGNAT IP (100.x); use a public hostname/IP for independent signaling")
	}
	if len(r.serverICE) == 0 {
		w = append(w, "no server ICE config from coordination server; run mynet login")
	} else if !hasTURNServer(r.serverICE) {
		w = append(w, "server did not advertise TURN; relay-only and some NAT scenarios will fail")
	} else if r.cfg.Network.IndependentMode {
		for _, s := range r.serverICE {
			for _, u := range s.URLs {
				if turnHostIsTailscale(u) {
					w = append(w, "server TURN URL uses a Tailscale address; external peers cannot use relay (set MYNET_TURN_URLS to a public IP/DNS name)")
					break
				}
			}
		}
	}
	return w
}

func turnHostIsTailscale(turnURL string) bool {
	lu := strings.ToLower(strings.TrimSpace(turnURL))
	lu = strings.TrimPrefix(lu, "turn:")
	lu = strings.TrimPrefix(lu, "turns:")
	if i := strings.Index(lu, "?"); i >= 0 {
		lu = lu[:i]
	}
	host := lu
	if strings.Contains(host, "@") {
		host = strings.Split(host, "@")[1]
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	ip := net.ParseIP(host)
	return ip != nil && isTailscaleCGNATIP(ip)
}

func (r *Runtime) probeServerHealth() error {
	u, err := url.Parse(r.cfg.ServerURL)
	if err != nil {
		return err
	}
	u.Path = "/healthz"
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(u.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthz http %d", resp.StatusCode)
	}
	return nil
}

func serverHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func isTailscaleCGNATIP(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	return ip4[0] == 100 && (ip4[1]&0xC0) == 64
}

func hasTURNServer(servers []control.ICEServer) bool {
	for _, s := range servers {
		for _, u := range s.URLs {
			lu := strings.ToLower(u)
			if strings.HasPrefix(lu, "turn:") || strings.HasPrefix(lu, "turns:") {
				return true
			}
		}
	}
	return false
}
