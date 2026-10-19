package daemon

import (
	"fmt"
	"time"

	"github.com/gcostello02/mynet/internal/netprobe"
)

// PingPeerICMP measures VPN reachability via one ICMP echo to the peer's VPN IPv4.
func (n *Node) PingPeerICMP(peerName string) (time.Duration, error) {
	if peerName == "" {
		return 0, fmt.Errorf("peer name required")
	}
	n.mu.Lock()
	if !n.running {
		n.mu.Unlock()
		return 0, fmt.Errorf("vpn not active")
	}
	var vpnIP string
	if p, ok := n.peers[peerName]; ok && p.VPNIP != "" {
		vpnIP = p.VPNIP
	}
	n.mu.Unlock()
	if vpnIP == "" {
		return 0, fmt.Errorf("unknown peer %q", peerName)
	}
	return netprobe.PingIPv4(vpnIP, 3*time.Second)
}
