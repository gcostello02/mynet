# MyNet V0.4 M2 — Secure device join

## Invitation format (v1)

JSON object (deliver **out-of-band**; treat as a password):

```json
{
  "v": 1,
  "server_url": "https://coordinator.example",
  "network_id": "default",
  "coordinator_fingerprint": "<64 hex sha256(coordinator ed25519 pubkey)>",
  "coordinator_public_key": "<optional base64; omit to pin fingerprint only>",
  "enrollment_token": "<one-time token from admin>"
}
```

CLI accepts JSON, `mynet-invite:<base64url>`, or raw base64url encoding.

## Coordinator verification (client)

1. Parse invitation; require **HTTPS** `server_url`.
2. `GET /v1/network/public` — match `network_id` and `coordinator_fingerprint`.
3. Generate 32-byte random **challenge**; `POST /v1/network/proof` with `{ "timestamp", "challenge" }`.
4. Verify Ed25519 signature on `mynet-coordinator-proof-v2` (v1 timestamp-only proof still accepted for compatibility).
5. Reject redirect to another host (HTTP client `CheckRedirect`).

## CLI

```bash
mynet join '<base64url-invitation>' --name my-laptop
mynet join --name my-laptop   # interactive paste
```

After join: `mynet service install` / `mynet service start` (not automatic).

## Admin CLI

```bash
mynet admin login --server https://coordinator.example --user admin
mynet admin invitations create --label laptop --max-uses 1
mynet admin devices list
mynet admin devices revoke <device_id>
```

Admin session: `~/.mynet/admin-session.json` (separate from device `session.json`).

## Revocation reconciliation

Daemon refreshes `GET /v1/vpn/config` every ~45s when VPN is up. Peers absent from config are removed from WireGuard and ICE caches. Auth failures clear device session. Offline peers cannot learn revocation until they reconnect.

## Staging acceptance

See `docs/m040-m2-staging.md`.
