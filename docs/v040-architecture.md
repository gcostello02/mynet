# MyNet V0.4 — Architecture audit & constraints

Audit of the repository as of post–V0.3.0 (always-on VPN, libexec macOS, systemd Linux). **No assumption that features are missing** — this documents what exists and what it implies for V0.4.

## Repository map

| Area | Location | Role |
|------|----------|------|
| Coordinator | `cmd/mynet-server` | HTTP API + WebSocket signaling hub |
| Client CLI + daemon | `cmd/mynet` | `init`, `login`, `up`/`down`, `service`, IPC `_daemon` |
| Control plane types | `internal/control` | REST/WS message structs |
| Server implementation | `internal/control/server` | Store, hub, VPN alloc, SQLite persist |
| Client runtime | `internal/client` | Signaling, ICE sessions, VPN register, diagnostics |
| Long-lived VPN | `internal/daemon` | Node.Up, peer workers, netwatch, IPC |
| Identity | `internal/identity` | Ed25519 device keys, canonical sign messages |
| WireGuard | `internal/wgengine`, `internal/wgbind`, `internal/wgbind` | TUN + ICE bind |
| ICE | `internal/ice` | Pion wrapper, STUN/TURN server list |
| Service install | `internal/service` | launchd, systemd, libexec, upgrade |
| Config | `internal/config` | `~/.mynet/config.json`, session token, network filters |

## Coordinator — existing capabilities

### HTTP endpoints (`internal/control/server/server.go`)

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| GET | `/healthz` | none | Liveness |
| POST | `/v1/enroll` | enrollment secret | Register device Ed25519 identity |
| POST | `/v1/auth` | signed challenge | Issue bearer token |
| GET | `/v1/peers` | bearer | Peer directory + `online` (WS connected) |
| GET | `/v1/ice-servers` | bearer | STUN/TURN URLs + ephemeral TURN creds |
| POST | `/v1/sessions` | bearer | Create ICE signaling session |
| GET | `/v1/ws` | bearer query/header | Signaling WebSocket |
| POST | `/v1/vpn/register` | bearer + WG key sig | Allocate VPN IPv4, store WG pubkey |
| GET | `/v1/vpn/config` | bearer | Local /32 + peer WG configs |

**Admin (M1):** `POST /v1/admin/setup`, `POST /v1/admin/login`, `POST /v1/admin/logout`, `GET /v1/admin/status`, `GET /v1/admin/devices`, `PATCH /v1/admin/devices/{id}`, `POST /v1/admin/devices/{id}/revoke`, `GET|POST /v1/admin/invitations`, `DELETE /v1/admin/invitations/{id}`, `PUT /v1/admin/network/legacy-enrollment`, `GET /v1/network/public`.

**Enrollment order (M1):** validate invitation/legacy gate → verify device signature → persist device → redeem invitation use.

**Still not in M1:** dashboard UI, client `mynet join`, coordinator install wizard.

### Persistence

- **SQLite** (`MYNET_DATA_PATH` / `--data`): `devices` table — identity, name, WG key, VPN IP, revoked flag (`internal/control/server/persist.go`).
- **In-memory:** signaling sessions (`Store.sessions`), WebSocket hub (`Hub.clients`).
- **Secrets (env):** `MYNET_SERVER_SECRET`, `MYNET_ENROLLMENT_SECRET`, optional `MYNET_TURN_SECRET`, `MYNET_VPN_CIDR`.

### VPN addressing

- Pool from `MYNET_VPN_CIDR` (default `172.31.240.0/20`).
- `AllocateVPNIPv4` assigns stable host per device; rejects duplicate WG keys across devices (`internal/control/server/allocate.go`).

### Multi-peer

- `GET /v1/vpn/config` returns all enrolled peers with WG metadata.
- Daemon starts **one worker per peer** (`internal/daemon/peers.go`).
- ICE session admission, supersede, and recovery logic already handle restarts and stale transports.

### Signaling & discovery

- Online = WebSocket registered in hub (`Hub.IsOnline`).
- Session creation notifies both peers via `session_ready`; ICE creds/candidates relayed only to session participant.

## Client — existing capabilities

### State layout (`~/.mynet`)

| File | Contents |
|------|----------|
| `identity.json` | Ed25519 keys, device_id, name (secrets on device) |
| `wg.json` | WireGuard private key (local) |
| `config.json` | `server_url`, STUN/TURN, `network` filters |
| `session.json` | Bearer token + expiry |
| `service.json` | Supervised install metadata (V0.3.0) |
| `mynetd.sock` / `mynetd.pid` | Daemon IPC |

### CLI flow (today)

1. `mynet init --name X [--server URL]`
2. `mynet login --enrollment-secret SECRET` → enroll (if needed) + auth + save session
3. Supervised: `mynet service install` → `service start` → `MYNET_RUN_VPN_AT_START` → `Node.Up`
4. Or manual: `mynet up` / `mynet down`

Also: `peers`, `connect`, `ping`, `diagnose`, `preflight`, `vpn register`, `config`, `service upgrade`, etc.

### Service layer (V0.3.0)

- macOS: `/usr/local/libexec/mynet/mynet`, launchd `com.mynet.daemon`
- Linux: systemd unit, capabilities for root reading user `~/.mynet`
- Upgrade: stage `.next`, `CommitSwap`, preflight blocks install while daemon running

## Authentication & enrollment model (today)

| Primitive | Behavior |
|-----------|----------|
| Device identity | Ed25519; `device_id` = first 128 bits of SHA-256(pubkey) |
| Enrollment | **Single shared** `MYNET_ENROLLMENT_SECRET` (hashed server-side); any holder can enroll new devices |
| Session | HMAC bearer from `MYNET_SERVER_SECRET`; ~24h TTL |
| WG register | Separate signed message binding WG pubkey to device |
| Revocation | `Store.RevokeDevice` + SQLite; **no public admin API**; revoked devices fail `GetDevice` on auth |
| Coordinator trust | Clients trust `server_url` (no pinning); TLS is operator responsibility |

There is **no separate admin user** — enrolled devices are peers with equal API access (list peers, create sessions).

## Deployment assumptions (production today)

- Coordinator URL e.g. `https://mesh.example.com` (TLS termination external to Go binary).
- `mynet-server` often on mini PC; **separate process** from `mynet-daemon`.
- Cloudflare Tunnel documented for **HTTPS/WSS only** — not UDP TURN (`docs/deployment-options.md`).
- coturn optional (`deploy/coturn/`).
- Enrollment secret manually distributed; documented in `deploy/minipc-server-start.md`.

## Architectural constraints for V0.4

1. **One network per server process** — `NetworkID` string exists but multi-tenant networks are not implemented.
2. **Enrollment is all-or-nothing** — shared secret, not per-invitation scopes or approval queue.
3. **Management APIs missing** — revoke/rename/invite require new authenticated admin surface.
4. **No embedded dashboard** — static UI must be added or served by `mynet-server`.
5. **Signaling sessions are ephemeral** — coordinator restart drops in-flight ICE sessions (clients recover).
6. **Token in session.json** — device credential; distinct from future admin session.
7. **CLI is the source of truth for client ops** — dashboard should call same backend APIs.
8. **Root-owned daemon** — installers must not regress V0.3.0 paths or spawn competing daemons (`ShouldSpawnDetachedDaemon`).
9. **ICE/VPN recovery is mature** — product work should not destabilize `internal/daemon` + `internal/client` admission without strong tests.

## Reuse summary (high value)

- Full coordinator signaling + VPN registration stack
- SQLite device persistence schema (extend, don’t replace)
- Client supervised lifecycle + upgrade
- Multi-peer daemon and diagnostics (`mynet diagnose`)
- Protocol docs (`docs/protocol.md`) as baseline for `/v1` evolution
