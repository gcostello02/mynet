# Architecture

## Overview

MyNet V0.1 separates **control plane** (HTTPS + WebSocket signaling) from **data plane** (direct UDP between peers after ICE nomination).

```text
┌─────────────┐       signaling (WS)        ┌──────────────────┐
│  mynet CLI  │◄──────────────────────────►│  mynet-server    │
│  + ICE      │       REST enroll/auth      │  (coordination)  │
└──────┬──────┘                             └──────────────────┘
       │
       │  UDP application frames (ping/test)
       ▼
┌─────────────┐
│  peer device│
└─────────────┘
```

The server never forwards application UDP payloads.

## Components

| Piece | Role |
|-------|------|
| `cmd/mynet-server` | Enrollment, bearer sessions, peer discovery, ICE signaling relay |
| `cmd/mynet` | Identity, control API client, Pion ICE agent, app framing |
| `internal/identity` | Ed25519 device keys, `device_id` = first 128 bits of SHA-256(public key) |
| `internal/control` | REST + WS message types |
| `internal/ice` (`icenet`) | Pion ICE wrapper, path diagnostics |
| `internal/peer` | Length-prefixed app frames (message-oriented reads) |

## Device registration

1. Operator sets `MYNET_ENROLLMENT_SECRET` on the server (hashed at startup; never logged).
2. Client generates keys locally (`mynet init`); private keys stay on device (`~/.mynet/identity.json`, mode `0600`).
3. `mynet login` sends a signed enroll payload + enrollment secret once, then obtains a bearer token via signed auth challenge.

Enrollment proves network membership; ongoing `POST /v1/auth` proves possession of the device private key.

## Authentication and trust

- **Server secret** (`MYNET_SERVER_SECRET`): HMAC for bearer tokens (24h TTL in V0.1).
- **Bearer token** required for peer list, session creation, and WebSocket signaling.
- Signaling messages are accepted only from session participants; candidates are relayed to the paired peer only.
- **Trust model**: clients trust the coordination server for peer authorization and signaling integrity. The server must not be compromised; V0.1 does not pin server keys on clients.

Revocation: `Store.RevokeDevice` marks devices revoked; tokens for revoked devices fail peer lookup. When `MYNET_DATA_PATH` or `--data` is set, enrolled devices are persisted in SQLite and survive server restarts.

## ICE negotiation

1. Initiator calls `POST /v1/sessions` → server notifies both peers via `session_ready` (controlling vs controlled role).
2. Peers exchange `ice_credentials` and trickle `ice_candidate` over WebSocket (server relay).
3. Each peer runs Pion ICE: host candidates (LAN) + server-reflexive (STUN).
4. Controlling peer `Dial`, controlled peer `Accept` → nominated UDP 5-tuple for app data.

## Direct packet transmission

Application messages use a small binary frame (`MN` magic, type, seq, payload). Reads are **one datagram per frame** to match ICE/UDP message semantics.

## Planned WireGuard milestone

- Userspace or kernel WireGuard over the nominated 5-tuple (or a dedicated TUN)
- Stable CGNAT-free addressing (e.g. `10.77.0.0/16` per network)
- `mynet up` / `mynet down` integrating TUN, routes, and DNS — **explicitly out of V0.1 scope**
