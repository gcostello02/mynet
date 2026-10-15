package config

import (
	"net"
	"net/url"
	"strings"

	"github.com/grantcostello/mynet/internal/control"
)

// EffectiveICEURLs returns STUN/TURN URLs for Pion ICE (deduped, order preserved).
func (c ClientConfig) EffectiveICEURLs() []string {
	if len(c.ICEURLs) > 0 {
		return append([]string(nil), c.ICEURLs...)
	}
	out := make([]string, 0, len(c.STUNURLs)+len(c.TURNURLs))
	seen := make(map[string]struct{})
	add := func(u string) {
		u = strings.TrimSpace(u)
		if u == "" {
			return
		}
		if _, ok := seen[u]; ok {
			return
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	for _, u := range c.STUNURLs {
		add(u)
	}
	for _, u := range c.TURNURLs {
		add(u)
	}
	return out
}

// LocalICEServers builds STUN/TURN server entries from client config.
func LocalICEServers(cfg ClientConfig) []control.ICEServer {
	if len(cfg.ICEURLs) > 0 {
		return []control.ICEServer{{URLs: append([]string(nil), cfg.ICEURLs...)}}
	}
	var out []control.ICEServer
	if len(cfg.STUNURLs) > 0 {
		out = append(out, control.ICEServer{URLs: append([]string(nil), cfg.STUNURLs...)})
	} else {
		out = append(out, control.ICEServer{URLs: DefaultClientConfig().STUNURLs})
	}
	if len(cfg.TURNURLs) > 0 {
		out = append(out, control.ICEServer{URLs: append([]string(nil), cfg.TURNURLs...)})
	}
	return out
}

// BuildICEServers merges local config with server-provided ICE servers (server entries appended).
func BuildICEServers(cfg ClientConfig, fromServer []control.ICEServer) []control.ICEServer {
	local := LocalICEServers(cfg)
	if len(fromServer) == 0 {
		return local
	}
	out := make([]control.ICEServer, 0, len(local)+len(fromServer))
	out = append(out, local...)
	out = append(out, fromServer...)
	return out
}

func serverHostFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := u.Hostname()
	if host == "" {
		return ""
	}
	return host
}

// isTailscaleCGNAT returns true if host is an IP in 100.64.0.0/10 (Tailscale default range).
func isTailscaleCGNAT(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	// 100.64.0.0/10
	return ip4[0] == 100 && (ip4[1]&0xC0) == 64
}
