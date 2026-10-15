package icenet

import "testing"

func TestCountCandidates(t *testing.T) {
	c := CountCandidates([]CandidateInfo{
		{Type: "host", Address: "10.0.0.1:9"},
		{Type: "server-reflexive (STUN)", Address: "1.2.3.4:5"},
		{Type: "relay (TURN)", Address: "1.2.3.4:6"},
	})
	if c.Host != 1 || c.Srflx != 1 || c.Relay != 1 || c.Total != 3 {
		t.Fatalf("counts: %+v", c)
	}
}
