package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/grantcostello/mynet/internal/control"
)

// ICEConfig supplies STUN/TURN URLs advertised to authenticated clients.
type ICEConfig struct {
	STUNURLs          []string
	TURNURLs          []string // base turn:/stun: URLs without credentials
	TURNSharedSecret  []byte
	TURNCredentialTTL time.Duration
}

func (s *Server) buildICEServers(deviceID string) control.ICEServersResponse {
	cfg := s.cfg.ICE
	out := control.ICEServersResponse{ICEServers: []control.ICEServer{}}
	stun := cfg.STUNURLs
	if len(stun) == 0 {
		stun = []string{"stun:stun.l.google.com:19302"}
	}
	out.ICEServers = append(out.ICEServers, control.ICEServer{URLs: stun})

	if len(cfg.TURNURLs) == 0 || len(cfg.TURNSharedSecret) == 0 {
		return out
	}
	ttl := cfg.TURNCredentialTTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	user, pass, _ := control.TURNCredentials(cfg.TURNSharedSecret, deviceID, ttl)
	out.ICEServers = append(out.ICEServers, control.ICEServer{
		URLs:       cfg.TURNURLs,
		Username:   user,
		Credential: pass,
	})
	return out
}

func (s *Server) handleICEServersAuth(w http.ResponseWriter, r *http.Request, deviceID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	dev, ok := s.store.GetDevice(deviceID)
	if !ok {
		httpError(w, http.StatusUnauthorized, "unknown device")
		return
	}
	resp := s.buildICEServers(dev.DeviceID)
	writeJSON(w, http.StatusOK, resp)
}

// ParseCSVEnv splits a comma-separated env value into trimmed non-empty entries.
func ParseCSVEnv(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
