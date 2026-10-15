# Testing

## Automated

```bash
make test
```

Includes:

- Identity persistence and signing
- Server enroll/auth/peer ACLs
- Local ICE pair ping (no external network)
- Full stack: HTTP server + two WebSocket clients + ICE + UDP ping

Race detector enabled via `make test`.

External STUN is **not** required for CI tests (`stun_urls` empty in ICE unit tests).

## Same-LAN manual test (MacBook + mini PC)

1. Run `mynet-server` on a host both devices can reach (LAN IP or laptop).
2. Set the same `MYNET_SERVER_SECRET` and `MYNET_ENROLLMENT_SECRET`.
3. `mynet init` / `mynet login` on both; keep `mynet login` running or re-run before connect so WebSocket stays up.
4. On initiator: `mynet connect --peer <other>` then `mynet ping`.
5. Expect `mynet status` to show `host` and private LAN addresses on both sides.

## Independent connectivity (V0.1.1)

See [independent-connectivity.md](independent-connectivity.md), [deployment-options.md](deployment-options.md), and [turn-testing.md](turn-testing.md). Requires a **publicly reachable** coordination URL (not Tailscale-only).

## Tailnet test (off-LAN, both on Tailscale)

Use this while Tailscale is still carrying control traffic; MyNet validates P2P over `100.x` host candidates.

1. Mini PC: `mynet-server` on `:8787` and `mynet login --stay`.
2. Mac (not on home LAN): `mynet config tailnet --server http://<minipc-100.x>:8787`, then `mynet login`.
3. `mynet connect --peer minipc` — expect `host` with `100.x` local/remote addresses.
4. See [tailscale.md](tailscale.md) for config fields (`allow_tailscale_candidates`, `turn_urls`).

## Hotspot / cross-network test

1. Put MacBook on phone hotspot; mini PC stays on home LAN.
2. Server must be reachable from both (public VPS, or server on mini PC with port forward — only if you explicitly choose to expose it).
3. Configure STUN in `~/.mynet/config.json` if defaults are blocked.
4. Success indicator: nominated pair includes `server-reflexive (STUN)` and reflexive/public IPs, with working `mynet ping`.
5. Symmetric NAT or UDP-blocking Wi‑Fi may fail until TURN is implemented — document failure mode (ICE failed / timeout).

## How to identify direct paths

| Observation | Meaning |
|-------------|---------|
| Local/remote addrs are RFC1918 on same subnet | Likely true LAN path |
| Candidate type `server-reflexive (STUN)` | STUN learned public/reflexive endpoint; may still be direct P2P |
| Server logs show only WS traffic | Good — no app payload relay |
| tcpdump on server during ping shows no peer UDP | Good |
| tcpdump on peer shows UDP between peer IPs | Confirms direct data path |

## Expected logs

Server:

```text
INFO device online device_id=...
INFO device offline device_id=...
```

Client: errors on stderr; `mynet status` for state.

## Troubleshooting

| Symptom | Likely cause |
|---------|----------------|
| `enrollment denied` | Wrong `MYNET_ENROLLMENT_SECRET` |
| `invalid token` | Re-run `mynet login` |
| Peer offline | Peer needs active WebSocket (`mynet login`) |
| ICE timeout | NAT/firewall, missing STUN, or peer offline |
| Ping fails but status Connected | App read loop / framing (check both sides connected) |

Distinguish **authorization** (401/403 on REST) from **signaling** (WS relay / session_ready missing) from **ICE** (`ICE failed` in CLI) from **NAT** (use TURN + `--relay-only` to validate relay path).
