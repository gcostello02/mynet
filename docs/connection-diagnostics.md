# Connection diagnostics (V0.2.8+)

## Timing

`mynet diagnose --peer <name>` includes **Connection timing** for the last VPN ICE attempt when the daemon has observed one. Durations are elapsed time from attempt start (monotonic), not wall-clock skew across machines.

Stages: signaling ready → session created → remote credentials → gather → ICE connect → data transport ready.

## Errors

- **Current error** — applies only while the peer data path is not ready.
- **Last failure** — most recent failed attempt (historical), kept for troubleshooting after recovery.
- **Last success** — timestamp when the peer link last completed ICE successfully.

## ICE fields

- **Transport state** — live Pion ICE agent state on the nominated peer link.
- **Negotiation idle** — VPN mode keeps global `icePath` empty; idle here does not mean the peer is down if transport state shows `connected`.

## `mynet down`

Stops WireGuard/TUN only. Signaling WebSocket should stay up; brief disconnects may still occur from the network and are retried by the signaling loop.
