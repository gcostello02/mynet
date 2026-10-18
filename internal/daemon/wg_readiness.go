package daemon

import (
	"context"
	"time"

	"github.com/gcostello02/mynet/internal/client"
	"github.com/gcostello02/mynet/internal/wgengine"
)

const wgHandshakePoll = 50 * time.Millisecond

// observeWireGuardReadiness polls WireGuard UAPI until handshake or traffic is observed.
func observeWireGuardReadiness(ctx context.Context, rt *client.Runtime, engine *wgengine.Engine, peerName, sessionID, pubKey string) {
	if engine == nil || rt == nil || pubKey == "" {
		return
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(30 * time.Second)
	}
	var baselineRx, baselineTx uint64
	if st, ok, _ := engine.PeerStatsByPublicKey(pubKey); ok {
		baselineRx, baselineTx = st.RxBytes, st.TxBytes
	}
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return
		}
		st, ok, err := engine.PeerStatsByPublicKey(pubKey)
		if err != nil || !ok {
			time.Sleep(wgHandshakePoll)
			continue
		}
		if !st.LastHandshake.IsZero() {
			rt.MarkConnStage(peerName, sessionID, client.StageWireGuardHandshake)
			if st.RxBytes > baselineRx || st.TxBytes > baselineTx {
				rt.MarkConnStage(peerName, sessionID, client.StageVPNDataUsable)
				return
			}
			// Handshake without app traffic yet still counts as WG ready.
			if time.Since(st.LastHandshake) < 2*time.Minute {
				rt.MarkConnStage(peerName, sessionID, client.StageVPNDataUsable)
				return
			}
		}
		time.Sleep(wgHandshakePoll)
	}
}
