# Architecture

MyNet separates a **control plane** (HTTPS + WebSocket to your coordinator) from a **data plane** (WireGuard packets sent directly between devices over an ICE-nominated UDP path). The coordinator never carries VPN traffic.

```text
                 ┌──────────────────────────────────────┐
                 │  Coordinator  (cmd/mynet-server)      │
                 │  enrollment · auth · signaling · VPN  │
                 │  address allocation · admin API · UI  │
                 └───────────────┬──────────────────────┘
                    HTTPS / WSS  │  control plane only
        ┌────────────────────────┴───────────────────────┐
        │                                                │
 ┌──────┴────────┐        WireGuard over ICE        ┌─────┴─────────┐
 │ Edge device A │◄────────────────────────────────►│ Edge device B │
 │ mynet daemon  │   direct UDP (STUN) or TURN relay│ mynet daemon  │
 └───────────────┘                                  └───────────────┘
```

## Components

| Piece | Location | Role |
|-------|----------|------|
| Coordinator | `cmd/mynet-server`, `internal/control/server` | HTTP API, WebSocket signaling hub, SQLite state, coordinator Ed25519 identity, VPN address allocation, embedded admin dashboard |
| Client CLI and daemon | `cmd/mynet`, `internal/daemon`, `internal/client` | Enrollment, signaling, ICE sessions, VPN lifecycle, IPC over a Unix socket |
| Identity | `internal/identity`, `internal/wgkeys` | Ed25519 device keys, WireGuard (Curve25519) keys |
| ICE | `internal/ice` | Pion ICE wrapper, STUN/TURN server list |
| WireGuard | `internal/wgengine`, `internal/wgbind` | `wireguard-go` device plus an `ICEBind` that carries WireGuard datagrams over ICE connections |
| TUN and routes | `internal/tundev` | TUN device, per-peer `/32` routes |
| Service install | `internal/service` | systemd (Linux) and launchd (macOS) units, upgrades |
| Dashboard | `web/admin`, `internal/dashboard` | React SPA embedded in the coordinator with `go:embed` |

## Identities and keys

| Key | Type | Stored | Purpose |
|-----|------|--------|---------|
| Device identity | Ed25519 | `~/.mynet/identity.json` (0600) | Enrollment and API authentication |
| WireGuard key | Curve25519 | `~/.mynet/wg.json` (0600) | Tunnel encryption; never sent to the coordinator |
| Coordinator identity | Ed25519 | `coordinator.ed25519` next to the SQLite DB (0600) | Proves the coordinator's identity to joining devices |

`device_id` is the first 16 bytes of `SHA-256(ed25519_public_key)`, hex-encoded (32 characters). Names are unique per network.

## Lifecycle of a device

1. **Invite.** An admin creates a one-time invitation (CLI or dashboard). It carries the coordinator URL, network ID, coordinator fingerprint and a one-time enrollment token. See [joining-devices.md](joining-devices.md).
2. **Join.** `mynet join` verifies the coordinator against the invitation's fingerprint, then enrolls the device with a signed request (`POST /v1/enroll`) and registers its WireGuard public key (`POST /v1/vpn/register`). The coordinator allocates a stable IPv4 address from `MYNET_VPN_CIDR` (default `172.31.240.0/20`).
3. **Authenticate.** The daemon proves possession of its Ed25519 key (`POST /v1/auth`, signed timestamp) and receives a bearer token (24 h TTL, renewed automatically before expiry).
4. **Signal.** Each device holds one WebSocket (`/v1/ws`) to the coordinator. The coordinator relays ICE credentials and candidates only between the two participants of a session.
5. **Connect.** For each peer, the device with the lexicographically lower `device_id` initiates a session (this avoids ICE glare). Peers exchange credentials and candidates, Pion ICE nominates a UDP path (host, STUN server-reflexive, or TURN relay candidates), and the connection is registered with `ICEBind`.
6. **Tunnel.** `wireguard-go` encrypts IP packets from the TUN interface and sends them through `ICEBind`. Each peer has a `/32` route to the TUN; default routes and DNS are not changed.
7. **Reconcile.** While the VPN is up, the daemon refetches `GET /v1/vpn/config` every 45 s. Peers missing from the config (revoked or removed) are dropped from WireGuard and ICE.

## Why WireGuard over ICE

`wireguard-go` sends and receives through a pluggable `conn.Bind`. `ICEBind` maps each WireGuard peer to one nominated `ice.Conn` and writes raw WireGuard datagrams to it, so:

- NAT traversal stays in Pion ICE, and WireGuard uses exactly the path ICE nominated.
- The WireGuard peer endpoint is a stable logical address (`<peer vpn ip>:51820`). ICE reconnects swap the underlying connection without reconfiguring the peer.
- No WireGuard packet format changes, so tunnel crypto is standard WireGuard.

Before nomination, Pion owns the UDP sockets for STUN checks, so nothing writes to the connection until ICE reports connected.

## Coordinator API

| Area | Endpoints |
|------|-----------|
| Health and public info | `GET /healthz`, `GET /v1/network/public`, `POST /v1/network/proof` |
| First-run setup | `GET /v1/setup/status`, `GET /v1/setup/bootstrap` (loopback only), `POST /v1/admin/setup` |
| Device | `POST /v1/enroll`, `POST /v1/auth`, `GET /v1/peers`, `GET /v1/ice-servers`, `POST /v1/sessions`, `GET /v1/ws`, `POST /v1/vpn/register`, `GET /v1/vpn/config` |
| Admin | `POST /v1/admin/login`, `POST /v1/admin/logout`, `POST /v1/admin/password`, `GET /v1/admin/status`, `GET /v1/admin/devices`, `PATCH /v1/admin/devices/{id}`, `POST /v1/admin/devices/{id}/revoke`, `GET\|POST /v1/admin/invitations`, `DELETE /v1/admin/invitations/{id}`, `PUT /v1/admin/network/legacy-enrollment` |

Signed messages use domain-separated prefixes: `mynet-enroll-v1`, `mynet-auth-v1`, `mynet-vpn-register-v1`, `mynet-coordinator-proof-v2`.

WebSocket messages (JSON): `session_ready`, `ice_credentials`, `ice_candidate`, `gathering_complete`, `session_close`.

## Limits

- IPv4 only: IPv4 TUN, ICE over UDP4, per-peer `/32` routes.
- One network per coordinator (the schema reserves `network_id` for later).
- Admin sessions are held in memory, so restarting the coordinator signs admins out. Device VPN sessions are unaffected.
- Revoked peers learn about revocation on their next config refresh, so a device that is offline learns when it reconnects.
- A device's VPN address stays reserved after revocation.
- Setting up TUN devices and routes needs root.

See also: [security.md](security.md), [networking.md](networking.md), [operations.md](operations.md).
