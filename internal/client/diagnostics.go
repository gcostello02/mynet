package client

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/grantcostello/mynet/internal/config"
	"github.com/grantcostello/mynet/internal/control"
	icepkg "github.com/grantcostello/mynet/internal/ice"
)

// Diagnostics is a detailed connectivity report for the CLI.
type Diagnostics struct {
	DeviceName            string
	PeerName              string
	ServerURL             string
	IndependentMode       bool
	Signaling             string
	Authentication        string
	CoordinationOK        bool
	PeerOnline            bool
	ICEState              string // live ICE agent/transport state when known
	ICETransportState     string
	ICENegotiationIdle    bool   // true when runtime has no active session path (VPN global idle)
	DataPathReady         bool
	Connection            string
	TURNUsed              bool
	LocalType             string
	RemoteType            string
	LocalAddr             string
	RemoteAddr            string
	RTT                   time.Duration
	LastComm              string
	SignalingReconnect    int
	SignalingLastMsg      string
	WireGuardState        string
	ICEGatherDuration     time.Duration
	LocalCandidateCounts  icepkg.CandidateCounts
	RemoteCandidateCounts icepkg.CandidateCounts
	ICEExcludeInterfaces  []string
	ICEExcludeCIDRs       []string
	ICEActiveServers      []string
	LocalCandidates       []icepkg.CandidateInfo
	RemoteCandidates      []icepkg.CandidateInfo
	TailscaleSignaling    string // No | Yes | Unverified
	TailscaleDataPath     string // No | Yes | Unverified
	Warnings              []string
	LastError             string
	HistoricalLastError   string
	HistoricalLastErrorAt string
	LastSuccessAt         string
	ConnTiming            ConnTimingSnapshot
	RecoveryCount         int // deprecated: use ICEFailedAttempts
	RecoveryTimeline      RecoveryTimeline
	ICEFailedAttempts     int
	TransportRecoveries   int
	VPNDataPathReady      bool
	ICESessionID          string
	WGPeerFingerprint     string
	WGAllowedIPs          []string
	WGLastHandshake       string
	WGTxBytes             uint64
	WGRxBytes             uint64
	BindHasActiveConn     bool
	BindSendPackets       uint64
	BindSendBytes         uint64
	BindSendNoTransport   uint64
	BindSendErrors        uint64
	BindRecvPackets        uint64
	BindRecvBytes          uint64
	BindRecvDropped        uint64
	BindConsecutiveErrors  int
	BindLastSendError      string
	BindLastSendErrorAt    string
	BindLastSendOKAt       string
	BindLastRecvAt         string
	RecoveryAttempts       int
	RecoveryState          string
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

	snap := r.SignalingSnapshot()
	d.Signaling = snap.State
	d.SignalingReconnect = snap.ReconnectAttempts
	d.SignalingLastMsg = snap.LastMessageType
	if snap.LastDisconnectReason != "" && snap.State != "Connected" {
		d.Warnings = append(d.Warnings, "signaling: "+snap.LastDisconnectReason)
	}
	if !snap.LastMessageAt.IsZero() && d.LastComm == "" {
		d.LastComm = snap.LastMessageAt.UTC().Format(time.RFC3339)
	}

	r.mu.Lock()
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
	d.LocalCandidates = append([]icepkg.CandidateInfo(nil), r.lastLocalCandidates...)
	d.RemoteCandidates = append([]icepkg.CandidateInfo(nil), r.lastRemoteCandidates...)
	d.LocalCandidateCounts = icepkg.CountCandidates(d.LocalCandidates)
	d.RemoteCandidateCounts = icepkg.CountCandidates(d.RemoteCandidates)
	d.ICEGatherDuration = r.lastGatherReport.Duration
	d.ICEExcludeInterfaces = append([]string(nil), r.lastGatherReport.ExcludePatterns...)
	d.ICEExcludeCIDRs = append([]string(nil), r.lastGatherReport.ExcludeCIDRs...)
	d.ICEActiveServers = append([]string(nil), r.lastGatherReport.ICEServerURLs...)
	if r.connState == StateConnected && !r.lastPingAt.IsZero() {
		d.LastComm = r.lastPingAt.UTC().Format(time.RFC3339)
	}
	globalICE := d.ICEState
	if globalICE == "" || globalICE == "idle" {
		d.ICENegotiationIdle = true
	}
	r.mu.Unlock()

	hist, histAt := r.historicalLastError()
	if hist != "" {
		d.HistoricalLastError = hist
		if !histAt.IsZero() {
			d.HistoricalLastErrorAt = histAt.UTC().Format(time.RFC3339)
		}
	}
	r.mu.Lock()
	if !r.lastSuccessAt.IsZero() {
		d.LastSuccessAt = r.lastSuccessAt.UTC().Format(time.RFC3339)
	}
	r.mu.Unlock()

	if d.IndependentMode {
		servers := config.FilterICEServers(r.cfg, config.BuildICEServers(r.cfg, r.serverICE))
		if len(servers) == 0 && hasTURNServer(r.serverICE) {
			d.Warnings = append(d.Warnings, "independent mode: server TURN URL uses an excluded address (e.g. Tailscale 100.x); no usable TURN from coordination server")
		}
	}

	d.Warnings = append(d.Warnings, r.independentWarnings()...)

	d.TailscaleSignaling, d.TailscaleDataPath = r.tailscaleDependency(d)
	d.Warnings = append(d.Warnings, tailscalePathWarnings(d)...)

	dataPathReady := false
	vpnDataReady := false
	if peerName != "" {
		if pt, ok := r.PeerConnTiming(peerName); ok {
			d.ConnTiming = pt.Current
		}
		if rt, ok := r.PeerRecoveryTimeline(peerName); ok {
			d.RecoveryTimeline = rt
			d.ICEFailedAttempts = rt.ICEAttemptsFailed
		}
		r.mu.Lock()
		if link, ok := r.peerLinks[peerName]; ok && link != nil {
			d.ICESessionID = link.SessionID
			path := link.Path
			if link.Session != nil {
				path = link.Session.LivePath()
			}
			if path.ConnectionState != "" {
				d.ICEState = path.ConnectionState
			}
			if path.Connection != "" {
				d.Connection = path.Connection
			}
			d.TURNUsed = path.TURNUsed
			d.LocalType = path.LocalType
			d.RemoteType = path.RemoteType
			d.LocalAddr = path.LocalAddr
			d.RemoteAddr = path.RemoteAddr
		}
		r.mu.Unlock()

		if live, ok := r.PeerLinkLivePath(peerName); ok {
			if live.ConnectionState != "" {
				d.ICETransportState = live.ConnectionState
				d.ICEState = live.ConnectionState
				d.ICENegotiationIdle = false
			}
			if live.Connection != "" {
				d.Connection = live.Connection
			}
		}
		if strings.EqualFold(d.ICETransportState, "connected") {
			r.mu.Lock()
			link, ok := r.peerLinks[peerName]
			dataPathReady = ok && link != nil && link.Conn != nil
			r.mu.Unlock()
		}
		if d.WGLastHandshake != "" {
			vpnDataReady = true
		}
		if stage, ok := d.ConnTiming.Stages[StageVPNDataUsable]; ok {
			vpnDataReady = true
			_ = stage
		}

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

	d.LastError = r.effectiveLastError(peerName, dataPathReady)
	d.DataPathReady = dataPathReady
	d.VPNDataPathReady = vpnDataReady || dataPathReady && d.WGLastHandshake != ""
	if d.ICETransportState == "" && d.ICEState != "" && d.ICEState != "idle" {
		d.ICETransportState = d.ICEState
	}

	return d, nil
}

// PeerConnTiming returns connection-establishment timing for a VPN peer.
func (r *Runtime) PeerConnTiming(peerName string) (PeerConnTiming, bool) {
	if r.connTiming == nil {
		return PeerConnTiming{}, false
	}
	return r.connTiming.snapshot(peerName)
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
