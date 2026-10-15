# Protocol (V0.1)

API prefix: `/v1`

## Device identifiers

- `device_id`: 32 hex chars (16 bytes) — `SHA-256(ed25519_public_key)[:16]`
- Friendly `name`: unique per network on the server

## Enrollment `POST /v1/enroll`

```json
{
  "device_id": "...",
  "name": "macbook",
  "public_key": "<base64 raw 32-byte key>",
  "timestamp": 1710000000,
  "signature": "<base64>",
  "enrollment_secret": "<plaintext; not stored server-side>"
}
```

Signed bytes:

```text
mynet-enroll-v1
<device_id>
<name>
<timestamp>
```

## Authentication `POST /v1/auth`

```json
{
  "device_id": "...",
  "timestamp": 1710000000,
  "signature": "<base64>"
}
```

Signed bytes:

```text
mynet-auth-v1
<device_id>
<timestamp>
```

Response:

```json
{ "token": "<payload>.<hmac>", "expires_at": "RFC3339" }
```

## Peers `GET /v1/peers`

`Authorization: Bearer <token>`

Returns `[{ "device_id", "name", "online" }]`.

## ICE servers `GET /v1/ice-servers`

`Authorization: Bearer <token>`

Returns STUN/TURN URLs for the client ICE agent. When the server has `MYNET_TURN_SECRET` and `MYNET_TURN_URLS`, includes time-limited TURN credentials (coturn `use-auth-secret` format).

```json
{
  "ice_servers": [
    { "urls": ["stun:stun.l.google.com:19302"] },
    { "urls": ["turn:host:3478?transport=udp"], "username": "…", "credential": "…" }
  ]
}
```

## Sessions `POST /v1/sessions`

```json
{ "peer_name": "minipc" }
```

Response includes `session_id`, `role` (`controlling` for initiator).

## WebSocket `GET /v1/ws?token=...`

JSON messages:

| `type` | Purpose |
|--------|---------|
| `session_ready` | Begin ICE for `session_id`; payload includes `role` |
| `ice_credentials` | `{ "ufrag", "pwd" }` |
| `ice_candidate` | `{ "candidate": "<marshal>" }` |
| `gathering_complete` | Optional gather-done hint (`gathering_complete` / `SignalGatherDone`) |
| `session_close` | Tear down signaling session |

Server validates `session_id` and relays to the other participant only.

## Application UDP framing (post-ICE)

| Offset | Field |
|--------|-------|
| 0–1 | `M` `N` |
| 2 | protocol version `1` |
| 3 | type: `1` ping, `2` pong, `3` test |
| 4–7 | sequence (big-endian uint32) |
| 8–9 | payload length |
| 10+ | payload |

## Versioning

- Identity file `version: 1`
- Bump API path (`/v2`) for breaking signaling changes
- App frame `protoV` byte for payload evolution
