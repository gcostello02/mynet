package icenet

import (
	"strings"
	"time"

	"github.com/pion/ice/v4"
)

// CandidateCounts groups local/remote candidates by type.
type CandidateCounts struct {
	Host  int
	Srflx int
	Relay int
	Prflx int
	Total int
}

// GatherReport summarizes one ICE gathering pass (no secrets).
type GatherReport struct {
	Duration        time.Duration
	AgentState      string
	Local           CandidateCounts
	ExcludePatterns []string
	ExcludeCIDRs    []string
	ICEServerURLs   []string
}

// CountCandidates tallies candidate lines or summaries.
func CountCandidates(cands []CandidateInfo) CandidateCounts {
	var c CandidateCounts
	for _, cand := range cands {
		c.Total++
		switch {
		case strings.Contains(cand.Type, "host"):
			c.Host++
		case strings.Contains(cand.Type, "server-reflexive"):
			c.Srflx++
		case strings.Contains(cand.Type, "relay"):
			c.Relay++
		case strings.Contains(cand.Type, "peer-reflexive"):
			c.Prflx++
		}
	}
	return c
}

// CountLocalAgentCandidates reads gathered locals from a Pion agent.
func CountLocalAgentCandidates(agent *ice.Agent) (CandidateCounts, error) {
	cands, err := SummarizeLocalCandidates(agent)
	if err != nil {
		return CandidateCounts{}, err
	}
	return CountCandidates(cands), nil
}
