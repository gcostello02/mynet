# Networking: reachability, NAT traversal and TURN

Devices need two things: a reachable **coordinator** (HTTPS and WebSocket) and a **UDP path** to each other. The coordinator doesn't carry VPN traffic, so only signaling needs to be reachable by everyone.

## Coordinator reachability

Pick one:

| Option | Notes |
|--------|-------|
| **Public VPS** | Simplest for devices on cellular or behind strict NAT. Terminate TLS with Caddy or nginx and proxy to the container's `127.0.0.1:8080`. |
| **Home server with a port forward** | Forward TCP 443 to your reverse proxy. Use dynamic DNS if your IP changes. Doesn't work behind CGNAT. |
| **Cloudflare Tunnel (or similar)** | Good for HTTPS and WSS signaling with no inbound ports. It doesn't carry UDP, so it can't replace TURN. |

Always use HTTPS: invitations and `mynet join` require it. Clients and invitations must use the exact public URL you passed to `mynet admin setup --public-url`.

## How a UDP path is found

Each device gathers ICE candidates: LAN addresses (host), public addresses learned from STUN (server-reflexive) and, if configured, TURN relay addresses. Pion ICE tries pairs and nominates one. On the same LAN you get a direct LAN path, across the internet typically a hole-punched direct path, and as a last resort a TURN relay. `mynet status` shows which, as `Direct P2P` or `Relayed (TURN)`.

STUN defaults to `stun:stun.l.google.com:19302`. Override on the coordinator:

```bash
MYNET_STUN_URLS=stun:stun.example.com:3478
```

Clients pick up server-provided STUN/TURN settings from `GET /v1/ice-servers`.

## TURN relay (optional)

Add a relay for networks where direct UDP can't be established (symmetric NAT, UDP-blocking Wi-Fi). Relayed traffic is still WireGuard-encrypted end to end.

The repo includes a coturn setup in [`deploy/coturn`](../deploy/coturn):

```bash
cd deploy/coturn
# In turnserver.conf, set static-auth-secret to a long random string
docker compose up -d
```

Open UDP/TCP 3478 and the relay range (`49152–49252` in the sample config) on a public IP. Then configure the coordinator with the same secret:

```bash
MYNET_TURN_SECRET=<same secret as static-auth-secret>
MYNET_TURN_URLS=turn:turn.example.com:3478?transport=udp
```

Restart the coordinator. Devices receive short-lived TURN credentials (coturn's time-limited HMAC format) from `GET /v1/ice-servers`, and the shared secret never reaches clients or browsers. TURN needs a public IP; it can't sit behind a tunnel that only carries HTTP.

### Testing the relay

```bash
mynet connect --peer <name> --relay-only
mynet status
```

Success shows `Connection: Relayed (TURN)` and `TURN Used: true`, and `mynet diagnose --peer <name>` lists a `relay` local candidate. With an unreachable or wrongly configured TURN server, `--relay-only` fails with `relay-only: no local TURN relay candidates` rather than reporting a false connection.

## Using Tailscale alongside MyNet

MyNet doesn't modify Tailscale. To keep the two independent, use **independent mode** on a device:

```bash
mynet config independent --server https://your-host.example
```

It excludes Tailscale's CGNAT range (`100.64.0.0/10`) and the `tailscale0` interface from ICE candidates, and ignores a `100.x` server URL for tailnet ICE. MyNet then never uses Tailscale for signaling or its data path.

The opposite, for development when you're off the LAN but both devices are on a tailnet:

```bash
mynet config tailnet --server http://<coordinator-tailscale-ip>:8080
```

This allows `100.x` host candidates. A `100.x` server URL enables them automatically. `mynet config show` prints the effective settings, and `mynet config set-server <url>` changes only the coordinator URL. Config lives in `~/.mynet/config.json` (`server_url`, `stun_urls`, `turn_urls`, `allow_tailscale_candidates`, and the independent-mode exclusions).

Typical home layouts keep Tailscale for emergency admin SSH and use MyNet for day-to-day access.

## Verifying a direct path

| Observation | Meaning |
|-------------|---------|
| `Direct P2P` in `mynet status` | UDP flows between the devices without a relay |
| Local and remote addresses on the same private subnet | LAN path |
| Candidate type `server-reflexive` | STUN-learned public address; may still be direct |
| `Relayed (TURN)` | Traffic goes through your TURN server |
| Coordinator logs show only HTTP/WebSocket traffic | Good, as no VPN payload passes through the coordinator |

## When there's no path

Work through the layers in order: **auth** (HTTP 401/403 means re-join or revoked), **signaling** (the peer must be online with a WebSocket, so check `mynet peers`), **ICE** (`mynet diagnose --peer <name>`), **NAT** (add TURN, test with `--relay-only`). See also [operations.md](operations.md).
