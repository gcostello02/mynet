# M1 — Coordinator admin & invitations (developer guide)

**Branch:** `feature/v040-m1-coordinator-admin`  
**Status:** M1 security review checkpoint — not deployed to production mesh.

## Security guarantees (M1)

- **No remote admin bootstrap** without loopback or `MYNET_SETUP_TOKEN`.
- **No default admin password**; setup rejected if devices already enrolled (V0.3 mesh safe).
- **Invitation tokens**: 256-bit random, SHA-256 hashed (`mynet-invitation-v1`). DB-backed enroll uses a **single SQLite transaction** (validate invitation → increment uses → insert device); failed enroll never consumes a use.
- **Coordinator identity**: Ed25519 keypair generated at setup; private key in `coordinator.ed25519` beside the DB (`0600`, directory `0700`). **Not** exposed via HTTP. Server **refuses to start** if setup is complete but the key file is missing or corrupt (no silent regeneration).
- **Coordinator proof**: `POST /v1/network/proof` with `{ "timestamp" }` returns an Ed25519 signature over `mynet-coordinator-proof-v1` (replay window = auth skew). M2 clients verify against the invitation fingerprint / public key.
- **Revocation**: `auth_epoch`, hub disconnect, auth/API/WS fail immediately; VPN peer list updates on next config fetch (client reconciliation in M2).

## Staging coordinator (isolated)

```bash
export MYNET_SERVER_SECRET="$(openssl rand -base64 48)"
export MYNET_DATA_PATH="/tmp/mynet-staging/coordinator.db"
export MYNET_SECURE_COOKIES=0   # set 1 behind HTTPS
# Optional remote setup (otherwise localhost-only):
# export MYNET_SETUP_TOKEN="$(openssl rand -base64 32)"

mkdir -p "$(dirname "$MYNET_DATA_PATH")"
go run ./cmd/mynet-server --listen 127.0.0.1:18787 --data "$MYNET_DATA_PATH"
```

### Bootstrap (localhost only)

```bash
curl -sS -X POST http://127.0.0.1:18787/v1/admin/setup \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"change-me-12chars","network_display_name":"staging","public_server_url":"https://staging.example"}'
```

Save `coordinator_fingerprint` from the response (also on `GET /v1/network/public` after setup).

### Admin login (session cookies)

```bash
curl -sS -c /tmp/mynet-admin.cookies -X POST http://127.0.0.1:18787/v1/admin/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"change-me-12chars"}'
```

Mutating admin requests with cookies require `X-Mynet-CSRF` matching the `mynet_csrf` cookie.

### Create invitation

`POST /v1/admin/invitations` — response includes one-time `enrollment_token` and `coordinator_fingerprint`.

### Legacy enrollment policy

| Scenario | Behavior |
|----------|----------|
| Fresh DB + admin setup | Invitation-only (`legacy_enrollment_enabled=false`) |
| V0.3 DB with devices + `MYNET_ENROLLMENT_SECRET` | Legacy auto-enabled in meta on coordinator start |
| Admin toggle | `PUT /v1/admin/network/legacy-enrollment` `{"enabled":true}` (requires env secret) |

Enroll body for invitations: `enrollment_token`, `coordinator_fingerprint`, plus standard signed device fields.

### Coordinator proof

```bash
curl -sS -X POST http://127.0.0.1:18787/v1/network/proof \
  -H 'Content-Type: application/json' \
  -d '{"timestamp":'$(date +%s)'}'
```

Verify `signature` with the coordinator public key from the invitation (independent of TLS).

### Backup

Back up **both** the SQLite file (`MYNET_DATA_PATH`) and `coordinator.ed25519` in the same directory. Restoring only the DB without the key file prevents the coordinator from starting.

### In-memory mode (tests)

When `DataPath` is empty, invitation enrollment is mutex-protected in process memory only (not durable). Production must use SQLite.

## M2 prerequisites

- Client compares invitation fingerprint to `GET /v1/network/public` and/or verifies `POST /v1/network/proof`.
- Implement revocation reconciliation per `docs/m2-revocation-contract.md`.

## Do not

- Point staging at `mesh.example.com` or copy production `devices.db`.
- Restart production daemons or coordinators as part of M1 testing.
