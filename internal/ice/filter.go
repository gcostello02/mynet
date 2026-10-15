package icenet

import (
	"net"
	"strings"
)

// IPAllowFilter returns true when the IP may be used for host candidates.
func IPAllowFilter(excludeNets []*net.IPNet) func(net.IP) bool {
	if len(excludeNets) == 0 {
		return func(ip net.IP) bool { return ip != nil }
	}
	return func(ip net.IP) bool {
		if ip == nil {
			return false
		}
		for _, n := range excludeNets {
			if n.Contains(ip) {
				return false
			}
		}
		return true
	}
}

// InterfaceAllowFilter returns true when the interface may be used for ICE.
// Patterns are exact names or prefix patterns ending with * (e.g. tailscale0, utun*).
func InterfaceAllowFilter(excludePatterns []string) func(string) bool {
	patterns := normalizePatterns(excludePatterns)
	if len(patterns) == 0 {
		return func(string) bool { return true }
	}
	return func(name string) bool {
		for _, p := range patterns {
			if matchPattern(p, name) {
				return false
			}
		}
		return true
	}
}

func normalizePatterns(in []string) []string {
	out := make([]string, 0, len(in))
	for _, p := range in {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func matchPattern(pattern, name string) bool {
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(name, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == name
}
