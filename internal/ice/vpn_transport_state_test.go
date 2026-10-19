package icenet

import (
	"testing"

	"github.com/pion/ice/v4"
)

func TestStateVPNTransportTerminal(t *testing.T) {
	if StateVPNTransportTerminal(ice.ConnectionStateDisconnected) {
		t.Fatal("disconnected must not tear down VPN transport")
	}
	if !StateVPNTransportTerminal(ice.ConnectionStateFailed) {
		t.Fatal("failed should tear down")
	}
	if !StateVPNTransportTerminal(ice.ConnectionStateClosed) {
		t.Fatal("closed should tear down")
	}
}
