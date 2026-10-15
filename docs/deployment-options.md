# Coordination server deployment options

MyNet clients need a **reachable coordination endpoint** (HTTP REST + WebSocket). Peer UDP data does not flow through the server unless TURN is used.

## Option A — Public VPS (recommended)

| Service | Ports | Notes |
|---------|-------|-------|
| `mynet-server` | TCP 443 (HTTPS) or 8787 | TLS via Caddy/nginx; WSS for signaling |
| coturn (optional) | UDP/TCP 3478, UDP 49152–49252 | Same host or separate; set `MYNET_TURN_URLS` to public IP/DNS |

**Pros:** Works from cellular hotspot, symmetric NAT, no home router changes.  
**Cons:** Small monthly cost; you operate TLS certs.

Example client config (after approval to provision):

```json
{
  "server_url": "https://mynet.example.com",
  "network": {
    "independent_mode": true,
    "exclude_candidate_cidrs": ["100.64.0.0/10"],
    "exclude_interfaces": ["tailscale0"]
  }
}
```

Use `mynet config independent --server https://mynet.example.com`.

## Option B — Home mini PC

| Requirement | Detail |
|-------------|--------|
| Port forward | TCP → mini PC for server port |
| DNS | Dynamic DNS if residential IP changes |
| TURN | UDP 3478 + relay range forwarded to coturn |

**Pros:** No VPS. **Cons:** CGNAT on some ISPs blocks inbound; TURN must also be reachable from the internet for Test C.

## Option C — Cloudflare Tunnel

Suitable for **HTTPS/WSS signaling only** (`cloudflared` → `mynet-server`).  
**Not** a substitute for TURN UDP relay. Pair with Option A TURN or home-forwarded coturn.

## Server flags

```bash
export MYNET_SERVER_SECRET='...'      # >= 32 bytes
export MYNET_ENROLLMENT_SECRET='...'
export MYNET_DATA_PATH=/var/lib/mynet/devices.db
export MYNET_TURN_SECRET='...'      # optional, matches coturn
export MYNET_TURN_URLS='turn:turn.example.com:3478?transport=udp'

mynet-server --listen :8787 --data /var/lib/mynet/devices.db
```

**Do not** expose enrollment over plaintext HTTP on the public internet.

## Approval required

This repository does **not** provision VPS, DNS, firewall, or router rules automatically. Prepare configs locally; apply after operator approval.
