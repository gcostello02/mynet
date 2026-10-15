# WireGuard over Pion ICE — V0.2 design

## Problem

MyNet V0.1 carries **application frames** (`MN` magic) on a Pion `ice.Conn` after ICE nomination. V0.2 must carry **WireGuard datagrams** on the same negotiated path without corrupting ICE STUN checks (which run inside Pion before nomination) and without opening a second UDP socket that bypasses ICE.

## Options considered

### A. Custom `wireguard-go` `conn.Bind` over ICE (chosen)

WireGuard’s `device.Device` sends and receives encrypted UDP payloads through `conn.Bind`. We implement `ICEBind` that:

- Maps each WireGuard peer public key to one nominated `ice.Conn`.
- `Send(bufs, ep)` writes raw WG packets to that peer’s ICE connection.
- `Open()` returns receive functions that demux inbound ICE reads into WireGuard by peer endpoint identity.

**Pros:** Official extension point (`conn.Bind`); no WG crypto changes; ICE lifecycle stays in Pion; one socket per peer pair after nomination.

**Cons:** Must replace ICE transport on path migration (new nomination → swap conn in bind); must not share ICE conn with V0.1 `MN` framing when VPN is up.

### B. Userspace encapsulation (WG inside MN frames)

Wrap WG packets in MyNet application frames.

**Pros:** Single demux layer.

**Cons:** Not WireGuard-on-the-wire; breaks interoperability; extra overhead; easy to get wrong for MTU/TCP; rejected.

### C. Parallel WireGuard UDP socket + ICE only for signaling

Run `StdNetBind` on a local UDP port and try to “sync” endpoints with ICE.

**Cons:** WireGuard would not use the ICE-nominated path; NAT traversal fails. Rejected.

## Chosen architecture

```
┌─────────────┐     TUN      ┌──────────────────┐   ICEBind    ┌──────────┐
│  OS stack   │◄────────────►│ wireguard-go     │◄────────────►│ ice.Conn │
│ 10.77.0.x   │              │ device.Device    │  raw WG UDP │ (Pion)   │
└─────────────┘              └──────────────────┘              └──────────┘
                                      ▲
                              coordination (HTTPS/WSS)
                              WG pubkey + VPN IP only
```

1. **Coordination server** allocates stable `vpn_ipv4`, stores `wg_public_key`, serves authorized peer list. Ed25519 device auth unchanged; WG keys registered via signed `POST /v1/vpn/register`.
2. **Daemon (`mynetd`)** holds the sole signaling WebSocket per device. CLI uses Unix domain socket IPC.
3. **ICE** session per peer (glare avoidance: lower `device_id` initiates). After `Connected`, register conn in `ICEBind` and configure WG peer with logical endpoint `vpn_ip:51820`.
4. **Routing:** per-peer `/32` routes to the TUN interface; no default route changes.
5. **Path changes:** ICE reconnect updates the bind’s conn map; WG peer endpoint string stays stable (VPN IP).

## Demultiplexing and keepalives

- Before nomination, Pion owns STUN/ICE on the underlying UDP sockets; the application must not write to `ice.Conn` until connected.
- After nomination, **only** WireGuard packets use `ice.Conn` (no `MN` frames on VPN links).
- Pion continues consent freshness on the ICE agent while the conn is open.

## Identity

| Purpose | Key type |
|---------|----------|
| Enrollment / API auth | Ed25519 (`identity.json`) |
| WireGuard tunnel | Curve25519 (`wg.json`) |

WG public keys are bound to device IDs server-side via signed registration messages.

## Limitations (V0.2)

- IPv4 TUN and ICE UDP4 only; no full `/16` route installed.
- TURN relay path not fully validated in production.
- No automatic multi-network isolation yet (schema reserves `network_id`).
- Privileged TUN/route setup requires root/admin; not performed in CI.

See also: [vpn-setup.md](vpn-setup.md), [vpn-security.md](vpn-security.md).
