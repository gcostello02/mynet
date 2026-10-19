package client

import (
	"testing"

	"github.com/gcostello02/mynet/internal/control"
)

// A new session_ready must replace a stale Connecting session (regression for already_active skip).
func TestNewSessionReadyReplacesStaleConnectingSession(t *testing.T) {
	rt := NewRuntime(t.TempDir(), nil)
	rt.SetVPNMode(true)
	rt.mu.Lock()
	rt.activeSession = "sess-a"
	rt.connState = StateConnecting
	rt.mu.Unlock()

	readyB := control.SessionReadyPayload{SessionID: "sess-b", PeerName: "macbook", Role: "controlled"}
	if !rt.beginIncomingICESession(readyB) {
		t.Fatal("expected new session to be accepted")
	}
	rt.mu.Lock()
	got := rt.activeSession
	rt.mu.Unlock()
	if got != "sess-b" {
		t.Fatalf("expected activeSession sess-b, got %q", got)
	}
}

func TestOutOfOrderCredentialsThenSessionReady(t *testing.T) {
	rt := NewRuntime(t.TempDir(), nil)
	rt.pendingSignals = make(map[string][]control.WSMessage)
	rt.signalHandlers = make(map[string]func(control.WSMessage))
	sid := "sess-ooo"
	rt.dispatchSignal(control.WSMessage{
		Type:      control.SignalICECredentials,
		SessionID: sid,
		Payload:   control.ICECredentialsPayload{Ufrag: "u", Pwd: "p"},
	})
	delivered := false
	rt.setSignalHandler(sid, func(msg control.WSMessage) {
		if msg.Type == control.SignalICECredentials {
			delivered = true
		}
	})
	if !delivered {
		t.Fatal("expected out-of-order credentials in pending queue")
	}
}
