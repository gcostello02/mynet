package icenet

import (
	"net"

	"github.com/gcostello02/mynet/internal/control"
)

// SessionConfig configures one ICE negotiation.
type SessionConfig struct {
	Controlling              bool
	ICEServers               []control.ICEServer
	AllowTailscaleCandidates bool // allow host candidates on Tailscale 100.64.0.0/10
	ExcludeCIDRs             []*net.IPNet
	ExcludeInterfaces        []string
	RelayOnly                bool // gather/nominate relay (TURN) candidates only
}
