# MyNet (V0.2)

MyNet is an open-source learning project inspired by Tailscale. **V0.1.x is not a VPN** — it proves authenticated peer discovery, ICE-based NAT traversal signaling, and **direct UDP application messaging** between devices without routing payloads through the coordination server. **V0.2** adds a **WireGuard mesh VPN** over the existing ICE transport, stable virtual IPs (`10.77.0.0/16`), TUN + per-peer `/32` routes, and a background daemon (`mynet up` / `mynet down`). V0.1.2 coordination over Cloudflare and independent mode are unchanged.

## Module path

Go module: `github.com/gcostello02/mynet` (rename-friendly: update `go.mod` and imports when you pick a permanent Git host).

## Current capabilities

- Coordination server (`mynet-server`) with device enrollment, signed session auth, peer listing, and WebSocket ICE signaling
- Client (`mynet`) with Ed25519 device identity, CLI, LAN + STUN candidate gathering (Pion ICE), and framed ping/test messages over the selected UDP path
- Integration tests: local ICE pair, signaling server + two peers, auth/enrollment tests

## VPN (V0.2)

```bash
sudo mynet up      # TUN + WireGuard over ICE (starts mynetd if needed)
mynet status
sudo mynet down
```

See [docs/vpn-setup.md](docs/vpn-setup.md) and [docs/wireguard-design.md](docs/wireguard-design.md).

## Limitations

- IPv4 only; per-peer `/32` routes (no default route takeover)
- Externally reachable TURN not fully validated
- TURN relay optional via server `GET /v1/ice-servers` + coturn ([docs/turn.md](docs/turn.md)); symmetric NAT may still fail without it
- Single logical network per server instance; minimal revocation API (in-memory store)
- Application payloads are **not** encrypted end-to-end (ICE checks authenticate the path, not app data)
- Cross-ISP / hotspot P2P not verified in CI (manual testing required)

## Prerequisites

- Go 1.22+ (developed with Go 1.27)
- Two devices for real-world tests (macOS + Linux supported)
- **Does not** modify Tailscale, firewall, DNS, or routes — uses ordinary UDP sockets only

## Build

```bash
make build
# binaries: bin/mynet, bin/mynet-server
```

## Quickstart (same machine / LAN lab)

**Terminal 1 — server**

```bash
export MYNET_SERVER_SECRET="$(openssl rand -hex 32)"
export MYNET_ENROLLMENT_SECRET="dev-enroll-secret"
./bin/mynet-server --listen :8080
```

**Terminal 2 — device A**

```bash
./bin/mynet init --name macbook --server http://127.0.0.1:8080
export MYNET_ENROLLMENT_SECRET="dev-enroll-secret"
./bin/mynet login
./bin/mynet peers
```

**Terminal 3 — device B**

```bash
./bin/mynet init --name minipc --server http://127.0.0.1:8080
export MYNET_ENROLLMENT_SECRET="dev-enroll-secret"
./bin/mynet login
```

**Connect (A initiates)**

```bash
# on macbook
./bin/mynet connect --peer minipc
./bin/mynet ping --peer minipc
./bin/mynet status
```

On `minipc`, run `mynet login` first so signaling is online; when `macbook` connects, `minipc` handles the incoming ICE session automatically.

### Cloudflare Tunnel + mini PC

1. Run `mynet-server` on the mini PC (e.g. `:8787`) with `MYNET_DATA_PATH` for persistence.
2. Point a Cloudflare published route at `http://127.0.0.1:8787` (e.g. `https://mesh.example.com`).
3. On **both** peers: `mynet config independent --server https://mesh.example.com` then `mynet login`.
4. On the mini PC (server + peer): `mynet login --stay` in one terminal.
5. From the remote peer: `mynet connect --peer minipc` then `mynet ping --peer minipc`.

Verify `curl https://mesh.example.com/healthz` returns `ok`. TURN must use a **public** `turn:IP:3478` or DNS name with UDP forwarded — not the Cloudflare HTTP hostname and not a `100.x` Tailscale address if clients are off-tailnet.

ICE servers are configurable in `~/.mynet/config.json` (`stun_urls`, `turn_urls`, or combined `ice_urls`). Default STUN: `stun:stun.l.google.com:19302`. For development over Tailscale while off-LAN, see [docs/tailscale.md](docs/tailscale.md).

## Commands

| Command | Description |
|---------|-------------|
| `mynet init --name <name>` | Create device identity under `~/.mynet/` |
| `mynet login` | Enroll (first time) + authenticate + open signaling WebSocket |
| `mynet peers` | List authorized peers and online status |
| `mynet connect --peer <name> [--relay-only]` | Start ICE signaling with peer (initiator) |
| `mynet diagnose [--peer <name>]` | Signaling, ICE, and path diagnostics |
| `mynet ping --peer <name>` | RTT over direct UDP app channel |
| `mynet status` | Device / peer / path diagnostics |
| `mynet config show \| tailnet \| independent \| set-server` | Server URL and connectivity mode |
| `mynet down` | Close signaling and peer UDP (no system VPN teardown) |

## Verifying the data path

- The coordination server relays **signaling only** (ICE ufrag/pwd + candidates), not application frames.
- After connect, `mynet status` reports nominated candidate types (e.g. `host` vs `server-reflexive (STUN)`) and local/remote socket addresses from the ICE connection.
- Candidate types indicate what was **selected**, not a cryptographic proof; confirm with LAN addresses on the same subnet, or STUN reflexive addresses when crossing NAT. Compare with packet capture if needed (see `docs/testing.md`).

## Documentation

- [docs/architecture.md](docs/architecture.md)
- [docs/protocol.md](docs/protocol.md)
- [docs/testing.md](docs/testing.md)
- [docs/tailscale.md](docs/tailscale.md)
- [docs/turn.md](docs/turn.md)
- [docs/deployment-options.md](docs/deployment-options.md)
- [docs/independent-connectivity.md](docs/independent-connectivity.md)
- [docs/turn-testing.md](docs/turn-testing.md)
- [docs/audit-v0.1.1.md](docs/audit-v0.1.1.md)

## License

TBD — add a license file before public release.
