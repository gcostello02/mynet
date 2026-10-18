package client

import (
	"testing"

	"github.com/gcostello02/mynet/internal/control"
)

func TestPendingICECredentialsDeliveredBeforeHandler(t *testing.T) {
	rt := NewRuntime(t.TempDir(), nil)
	rt.pendingSignals = make(map[string][]control.WSMessage)
	rt.signalHandlers = make(map[string]func(control.WSMessage))

	sid := "sess-pending-cred"
	rt.dispatchSignal(control.WSMessage{
		Type:      control.SignalICECredentials,
		SessionID: sid,
		Payload:   control.ICECredentialsPayload{Ufrag: "u", Pwd: "p"},
	})

	got := false
	rt.setSignalHandler(sid, func(msg control.WSMessage) {
		if msg.Type == control.SignalICECredentials {
			got = true
		}
	})
	if !got {
		t.Fatal("expected queued ice_credentials to be delivered when handler registered")
	}
}
