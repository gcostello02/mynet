# M1 — Coordinator admin & invitations (developer guide)

**Branch:** `feature/v040-m1-coordinator-admin`  
**Status:** M1 security review checkpoint — not deployed to production mesh.

## Security guarantees (M1)

- **No remote admin bootstrap** without loopback or `MYNET_SETUP_TOKEN`.
- **No default admin password**; setup rejected if devices already enrolled (V0.3 mesh safe).
- **Invitation tokens**: 256-bit random, SHA-256 hashed (`mynet-invitation-v1`), redeemed **after** successful enroll.
- **Coordinator fingerprint**: SHA-256(coordinator Ed25519 public key); private key **not stored** after setup.
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

## M2 prerequisites

- Client compares invitation fingerprint to `GET /v1/network/public` over TLS (out-of-band pin).
- Implement revocation reconciliation per `docs/m2-revocation-contract.md`.

## Do not

- Point staging at `mesh.example.com` or copy production `devices.db`.
- Restart production daemons or coordinators as part of M1 testing.
