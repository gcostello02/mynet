package config

import (
	"net"
	"strings"

	"github.com/gcostello02/mynet/internal/control"
)

// FilterICEServers removes STUN/TURN URLs that resolve to excluded address ranges in independent mode.
func FilterICEServers(cfg ClientConfig, servers []control.ICEServer) []control.ICEServer {
	if !cfg.Network.IndependentMode {
		return servers
	}
	nets := cfg.ParsedExcludeCIDRs()
	var out []control.ICEServer
	for _, s := range servers {
		var urls []string
		for _, u := range s.URLs {
			if iceURLPermitted(u, nets) {
				urls = append(urls, u)
			}
		}
		if len(urls) == 0 {
			continue
		}
		out = append(out, control.ICEServer{
			URLs:       urls,
			Username:   s.Username,
			Credential: s.Credential,
		})
	}
	return out
}

func iceURLPermitted(raw string, exclude []*net.IPNet) bool {
	host := iceURLHost(raw)
	if host == "" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		// Public DNS name — allow (future public TURN).
		return true
	}
	for _, n := range exclude {
		if n.Contains(ip) {
			return false
		}
	}
	return true
}

func iceURLHost(raw string) string {
	lu := strings.ToLower(strings.TrimSpace(raw))
	for _, prefix := range []string{"stun:", "stuns:", "turn:", "turns:"} {
		if strings.HasPrefix(lu, prefix) {
			lu = strings.TrimPrefix(lu, prefix)
			break
		}
	}
	if i := strings.Index(lu, "?"); i >= 0 {
		lu = lu[:i]
	}
	host := lu
	if strings.Contains(host, "@") {
		host = strings.Split(host, "@")[1]
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}
