package icenet

import (
	"strings"

	"github.com/pion/ice/v4"
)

// CandidateInfo is a sanitized ICE candidate for diagnostics (no secrets).
type CandidateInfo struct {
	Type    string
	Address string
}

// SummarizeLocalCandidates returns gathered local candidates.
func SummarizeLocalCandidates(agent *ice.Agent) ([]CandidateInfo, error) {
	cands, err := agent.GetLocalCandidates()
	if err != nil {
		return nil, err
	}
	out := make([]CandidateInfo, 0, len(cands))
	for _, c := range cands {
		if c == nil {
			continue
		}
		out = append(out, CandidateInfo{
			Type:    candidateTypeLabel(c.Type()),
			Address: c.Address(),
		})
	}
	return out, nil
}

// SummarizeRemoteCandidate parses a marshaled remote candidate line.
func SummarizeRemoteCandidate(marshaled string) CandidateInfo {
	typ := "unknown"
	lower := strings.ToLower(marshaled)
	switch {
	case strings.Contains(lower, " typ relay "):
		typ = candidateTypeLabel(ice.CandidateTypeRelay)
	case strings.Contains(lower, " typ srflx "):
		typ = candidateTypeLabel(ice.CandidateTypeServerReflexive)
	case strings.Contains(lower, " typ prflx "):
		typ = candidateTypeLabel(ice.CandidateTypePeerReflexive)
	case strings.Contains(lower, " typ host "):
		typ = candidateTypeLabel(ice.CandidateTypeHost)
	}
	addr := parseCandidateAddress(marshaled)
	return CandidateInfo{Type: typ, Address: addr}
}

func parseCandidateAddress(line string) string {
	// candidate:... <priority> <ip> <port> typ ...
	parts := strings.Fields(line)
	if len(parts) >= 6 {
		return parts[4] + ":" + parts[5]
	}
	return ""
}
