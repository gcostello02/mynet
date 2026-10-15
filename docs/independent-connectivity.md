# Independent connectivity (without Tailscale)

Tailscale may remain installed for emergency access. MyNet adds configuration to avoid using Tailscale for **signaling** or **ICE host candidates** during tests.

## Cloudflare Tunnel (coordination only)

Publish the coordination server HTTP API to the internet; **do not** route TURN UDP through the same HTTP hostname.

| Check | Command |
|-------|---------|
| Health | `curl -i https://YOUR_HOST/healthz` → `200 ok` |
| Auth | `mynet login` against `https://YOUR_HOST` |
| WSS | `mynet login` opens `wss://YOUR_HOST/v1/ws?token=…` (automatic from `https://`) |

Cloudflare proxies **HTTPS and WSS** to your origin (`http://127.0.0.1:8787`). ICE **UDP** and **TURN** still require reachable public IPs, port forwards, or IPv6 — not the tunnel hostname.

## Client setup

```bash
mynet config independent --server https://YOUR_PUBLIC_COORDINATION_HOST
mynet login --enrollment-secret "$MYNET_ENROLLMENT_SECRET"
mynet login --stay   # on always-on peer (mini PC)
```

Run **one** signaling session per device. A second `mynet login` or `mynet connect` from another terminal replaces the WebSocket on the server and can interrupt an in-flight ICE session.

`independent_mode` enables:

- `exclude_candidate_cidrs`: `100.64.0.0/10` (Tailscale CGNAT)
- `exclude_interfaces`: `tailscale0` (Linux; adjust on macOS if needed)
- No auto tailnet ICE from a `100.x` server URL

## Validation procedure

1. **Signaling:** `mynet diagnose --peer minipc` — server URL must not be a Tailscale-only address; `Signaling: Connected` after login.
2. **STUN/TURN:** After login, warnings should not say TURN is missing if relay tests are planned.
3. **ICE path:** After `mynet connect --peer minipc`, `mynet status` shows `Connection: Direct P2P` or `Relayed (TURN)` and `TURN Used: true/false`.
4. **Relay test:** `mynet connect --peer minipc --relay-only` — must fail if TURN is unreachable; must show `TURN Used: true` when successful.
5. **Packet capture (optional):** On TURN success, UDP flows to the TURN server IP on 3478/relay ports during connect.

MyNet does **not** modify system routing tables or Tailscale settings.

## Hotspot test (Test B)

1. Mini PC: server + coturn + `mynet login --stay` on home network with port forward or VPS.
2. Mac: phone hotspot, `mynet config independent`, login, `mynet connect --peer minipc`.
3. Expect STUN reflexive or host (if hairpin) paths; otherwise configure TURN.

See [deployment-options.md](deployment-options.md) and [turn-testing.md](turn-testing.md).
