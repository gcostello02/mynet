package icenet

import (
	"fmt"
	"strings"

	"github.com/grantcostello/mynet/internal/control"
	"github.com/pion/ice/v4"
)

func pionURLsFromICEServers(servers []control.ICEServer) ([]*ice.URL, error) {
	out := make([]*ice.URL, 0, len(servers))
	for _, srv := range servers {
		for _, raw := range srv.URLs {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			parsed, err := ice.ParseURL(raw)
			if err != nil {
				return nil, fmt.Errorf("parse ice url %q: %w", raw, err)
			}
			if srv.Username != "" {
				parsed.Username = srv.Username
				parsed.Password = srv.Credential
			}
			out = append(out, parsed)
		}
	}
	return out, nil
}
