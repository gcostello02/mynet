// Package icenet wraps Pion ICE for MyNet peer connectivity (not the Go std net/ice).
package icenet

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gcostello02/mynet/internal/control"
	"github.com/pion/ice/v4"
)

// PathInfo describes the selected ICE candidate pair for diagnostics.
type PathInfo struct {
	LocalAddr       string
	RemoteAddr      string
	LocalType       string
	RemoteType      string
	Transport       string
	Connection      string // "Direct P2P" or "Relayed (TURN)"
	TURNUsed        bool
	Nominated       bool
	ConnectionState string
}

// Session runs ICE gathering and connectivity for one peer link.
type Session struct {
	agent          *ice.Agent
	controlling    bool
	iceURLs        []string
	allowTailscale bool

	mu          sync.Mutex
	conn        *ice.Conn
	path        PathInfo
	state       ice.ConnectionState
	onCandidate func(candidate string)
}

// NewSession creates an ICE agent from SessionConfig.
func NewSession(cfg SessionConfig) (*Session, error) {
	opts := []ice.AgentOption{
		ice.WithNetworkTypes([]ice.NetworkType{ice.NetworkTypeUDP4}),
		ice.WithMulticastDNSMode(ice.MulticastDNSModeDisabled),
	}
	ipFilter := IPAllowFilter(cfg.ExcludeCIDRs)
	if !cfg.AllowTailscaleCandidates && len(cfg.ExcludeCIDRs) == 0 {
		ipFilter = lanCandidateIPFilter
	}
	opts = append(opts, ice.WithIPFilter(ipFilter))
	if len(cfg.ExcludeInterfaces) > 0 {
		opts = append(opts, ice.WithInterfaceFilter(InterfaceAllowFilter(cfg.ExcludeInterfaces)))
	}
	if cfg.RelayOnly {
		opts = append(opts, ice.WithCandidateTypes([]ice.CandidateType{ice.CandidateTypeRelay}))
	}
	if len(cfg.ICEServers) > 0 {
		urls, err := pionURLsFromICEServers(cfg.ICEServers)
		if err != nil {
			return nil, err
		}
		opts = append(opts, ice.WithUrls(urls))
	}
	agent, err := ice.NewAgentWithOptions(opts...)
	if err != nil {
		return nil, err
	}
	s := &Session{
		agent:          agent,
		controlling:    cfg.Controlling,
		iceURLs:        iceURLStrings(cfg.ICEServers),
		allowTailscale: cfg.AllowTailscaleCandidates,
		state:          ice.ConnectionStateNew,
	}
	if err := agent.OnConnectionStateChange(func(st ice.ConnectionState) {
		s.mu.Lock()
		s.state = st
		s.path.ConnectionState = st.String()
		s.mu.Unlock()
	}); err != nil {
		_ = agent.Close()
		return nil, err
	}
	return s, nil
}

func iceURLStrings(servers []control.ICEServer) []string {
	out := make([]string, 0, len(servers))
	for _, s := range servers {
		out = append(out, s.URLs...)
	}
	return out
}

// SetCandidateHandler receives marshaled remote candidates.
func (s *Session) SetCandidateHandler(fn func(candidate string)) error {
	return s.agent.OnCandidate(func(c ice.Candidate) {
		if c == nil {
			if fn != nil {
				fn("")
			}
			return
		}
		if fn != nil {
			fn(c.Marshal())
		}
	})
}

// LocalCredentials returns ICE ufrag and password for the peer.
func (s *Session) LocalCredentials() (ufrag, pwd string, err error) {
	return s.agent.GetLocalUserCredentials()
}

// AddRemoteCandidate unmarshals and adds a remote ICE candidate.
func (s *Session) AddRemoteCandidate(marshaled string) error {
	if marshaled == "" {
		return nil
	}
	c, err := ice.UnmarshalCandidate(marshaled)
	if err != nil {
		return err
	}
	return s.agent.AddRemoteCandidate(c)
}

// Gather starts candidate gathering.
func (s *Session) Gather() error {
	return s.agent.GatherCandidates()
}

// Connect completes ICE and returns the data conn. Blocks until connected or ctx canceled.
func (s *Session) Connect(ctx context.Context, remoteUfrag, remotePwd string) (*ice.Conn, error) {
	var conn *ice.Conn
	var err error
	if s.controlling {
		conn, err = s.agent.Dial(ctx, remoteUfrag, remotePwd)
	} else {
		conn, err = s.agent.Accept(ctx, remoteUfrag, remotePwd)
	}
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.conn = conn
	s.path = buildPathInfo(conn)
	s.mu.Unlock()
	return conn, nil
}

// LocalCandidates returns summaries of gathered local candidates.
func (s *Session) LocalCandidates() ([]CandidateInfo, error) {
	return SummarizeLocalCandidates(s.agent)
}

// AgentState returns the ICE agent connection state string.
func (s *Session) AgentState() string {
	return s.State().String()
}

// Path returns diagnostics for the nominated pair when available.
func (s *Session) Path() PathInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn != nil {
		s.path = buildPathInfo(s.conn)
	}
	return s.path
}

// State returns the ICE agent connection state.
func (s *Session) State() ice.ConnectionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// Close shuts down the ICE agent.
func (s *Session) Close() error {
	return s.agent.Close()
}

func buildPathInfo(conn *ice.Conn) PathInfo {
	local := conn.LocalAddr()
	remote := conn.RemoteAddr()
	info := PathInfo{
		LocalAddr:       addrString(local),
		RemoteAddr:      addrString(remote),
		Transport:       "UDP",
		ConnectionState: ice.ConnectionStateConnected.String(),
	}
	for _, pair := range conn.GetCandidatePairsInfo() {
		if pair.Nominated {
			info.Nominated = true
			info.LocalType = candidateTypeLabel(pair.LocalCandidateType)
			info.RemoteType = candidateTypeLabel(pair.RemoteCandidateType)
			info.TURNUsed = pair.LocalCandidateType == ice.CandidateTypeRelay || pair.RemoteCandidateType == ice.CandidateTypeRelay
			break
		}
	}
	if !info.Nominated {
		info.LocalType = "unknown"
		info.RemoteType = "unknown"
	}
	if info.TURNUsed {
		info.Connection = "Relayed (TURN)"
	} else {
		info.Connection = "Direct P2P"
	}
	return info
}

func addrString(a net.Addr) string {
	if a == nil {
		return ""
	}
	return a.String()
}

func candidateTypeLabel(t ice.CandidateType) string {
	switch t {
	case ice.CandidateTypeHost:
		return "host"
	case ice.CandidateTypeServerReflexive:
		return "server-reflexive (STUN)"
	case ice.CandidateTypePeerReflexive:
		return "peer-reflexive"
	case ice.CandidateTypeRelay:
		return "relay (TURN)"
	default:
		return strings.ToLower(t.String())
	}
}

// ConnectTimeout is the default ICE dial timeout.
const ConnectTimeout = 30 * time.Second

// lanCandidateIPFilter drops Tailscale CGNAT (100.64.0.0/10) host addresses so LAN peers pick reachable 10.x/192.x paths.
func lanCandidateIPFilter(ip net.IP) bool {
	if ip == nil {
		return false
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return true
	}
	return ip4[0] != 100
}
