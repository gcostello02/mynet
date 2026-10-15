# V0.1.1 repository audit (pre-implementation)

Date: 2026-03-13. Scope: independent P2P without Tailscale for signaling or data.

## Architecture summary

- **Control plane:** HTTP REST + WebSocket on `mynet-server`; bearer HMAC tokens; Ed25519 enroll/auth.
- **Data plane:** Pion ICE UDP4; app frames via `internal/peer` (not encrypted).
- **ICE servers:** Client merges `~/.mynet/config.json` with `GET /v1/ice-servers` (STUN + ephemeral TURN when server env set).
- **Tailscale coupling (prior to V0.1.1):** Optional dev path via `100.x` server URL and `allow_tailscale_candidates`; not required by code.

## Risks

| Risk | Severity | Mitigation in V0.1.1 |
|------|----------|----------------------|
| Server only on LAN/tailnet IP | High | `deployment-options.md`; `config independent` |
| In-memory device store lost on restart | Medium | SQLite persistence (`MYNET_DATA_DIR`) |
| Plain HTTP signaling | High | Document TLS requirement; examples use `https://` |
| WS token in query string | Medium | Document; prefer `Authorization` header where possible |
| TURN not validated on real NAT | High | `turn-testing.md`, `--relay-only`, diagnostics |
| Accidental `100.x` ICE while testing independence | Medium | `independent_mode`, CIDR filters |
| No app-layer encryption | Info | Document; WireGuard later |

## Recommended independent deployment

**Option A (VPS)** is the simplest reliable choice for Test B (hotspot): public HTTPS/WSS on 443, UDP 3478 (+ relay ports) for TURN. Home-hosted (Option B) requires port forward and dynamic DNS. Cloudflare Tunnel (Option C) is suitable for **signaling only**, not TURN UDP.

No cloud resources are provisioned by the repository; see `docs/deployment-options.md`.

## Preserved behavior

- LAN `10.x` host candidates when not in independent mode.
- Existing CLI commands and enroll/auth flow.
- Tailscale left untouched on the OS.
