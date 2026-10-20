# M3.3 — Self-hosted first-run setup

Branch: `feature/v040-m3-dashboard`  
Prior dashboard revision: `11d1fb6` (premium UI).

## Task 1 — Audit: reusable components

| Area | Existing behavior | Reuse in M3.3 |
|------|-------------------|---------------|
| `POST /v1/admin/setup` | Creates first admin, Ed25519 identity, SQLite meta, invitation-only default | **Kept**; now atomic transaction + bootstrap gate for remote clients |
| Loopback setup | `isLoopback(r)` allows setup without extra proof | **Preserved** for on-host CLI/scripts (`bootstrap-coordinator.sh` via `docker exec` curl to 127.0.0.1) |
| `MYNET_SETUP_TOKEN` + `X-Mynet-Setup-Token` | Optional static remote token | **Preserved** for automation; one-time bootstrap capability preferred for browsers |
| SQLite + migrations | `network_meta`, `admins`, `devices`, `invitations` | **Unchanged schema**; bootstrap hash stored in `network_meta` until consumed |
| Coordinator identity | `GenerateCoordinatorIdentity`, `coordinator.ed25519` on disk | **Unchanged** |
| Dashboard auth | HttpOnly session, CSRF on mutations, same-origin | **Unchanged**; wizard uses setup + login only |
| Invitations | `POST /v1/admin/invitations` one-time token | **Wizard step 5–6** |
| Client join | `mynet join '<token>' --name …`, fingerprint pin | **Documented in wizard** |
| Staging bootstrap | `deploy/staging/scripts/bootstrap-coordinator.sh` | **Still valid** (loopback setup inside container) |

Not weakened: production V0.3 meshes with existing devices still block admin setup; ICE/WireGuard paths untouched.

## Task 2 — Secure browser bootstrap

### Threat model (summary)

| Threat | Mitigation |
|--------|------------|
| Internet attacker initializes your network | Remote `POST /v1/admin/setup` requires one-time bootstrap capability (or legacy setup token); capability only issued on uninitialized DB and shown on host |
| Bootstrap replay | Capability cleared on successful setup; second setup returns 403 |
| Concurrent double setup | `setupMu` + SQLite transaction with `setup_complete` / admin count checks |
| Secret in logs/URLs | Printed only to stderr banner; `GET /v1/setup/bootstrap` loopback-only; header `X-Mynet-Bootstrap-Token`; `Cache-Control: no-store` on setup routes |
| Predictable bootstrap | `newRandomToken(32)` (256-bit), hashed with domain-separated SHA-256 |
| Host ownership via spoofed headers | Bootstrap not derived from install ID; loopback uses `RemoteAddr`; no trust in `X-Forwarded-For` for bootstrap issuance |

### API

| Method | Path | Auth | Notes |
|--------|------|------|-------|
| GET | `/v1/setup/status` | None | `initialized`, `bootstrap_active`, `bootstrap_expires_at` |
| GET | `/v1/setup/bootstrap` | Loopback only | Returns `bootstrap_token` + expiry |
| POST | `/v1/admin/setup` | Loopback, bootstrap header, or setup token | Body: username, password, network name, public URL |

Default bootstrap TTL: **2 hours** (new capability on each start until setup completes).

## Task 3 — Guided setup wizard

React route flow (`SetupWizardPage`):

1. Welcome (self-hosted explanation)
2. Bootstrap capability (paste or “Load from coordinator host” via loopback proxy)
3. Network name + admin credentials + public URL → `POST /v1/admin/setup` + auto-login
4. Coordinator fingerprint + backup guidance
5. First invitation (real `POST /v1/admin/invitations`)
6. Join instructions (macOS/Linux), fingerprint verification, service start; polls devices list

State: bootstrap secret and invitation token live **only in component memory** (not `localStorage`).

## Task 4 — Deployment UX

- `deploy/self-hosted/docker-compose.yml` — single coordinator, persistent volume, localhost bind, healthcheck
- `deploy/self-hosted/.env.example` — secrets, STUN/TURN, secure cookies
- `deploy/self-hosted/README.md` — backup, upgrade, HTTPS expectations

Admin is not intentionally exposed on `0.0.0.0` in the example compose file; operators front with HTTPS reverse proxy.

## Task 5 — First-device onboarding

Covered in wizard step 6: one-time invitation display, `mynet join` command, fingerprint check, `mynet service start`, enrollment poll. No browser-driven root/service changes.

## Task 6 — Tests

Go (`internal/control/server/bootstrap_test.go`):

- Uninitialized status
- Remote setup without bootstrap → 403
- Remote setup with bootstrap → 201, replay → 403
- Loopback bootstrap endpoint
- Concurrent setup (one winner)

Existing M1/M3 browser security and admin flow tests still apply.

## Task 7 — Staging validation (operator, no auto-deploy)

On a **disposable** machine (not production/staging mesh):

```bash
cd deploy/self-hosted
cp .env.example .env   # set MYNET_SERVER_SECRET
docker compose build && docker compose up -d
docker compose logs coordinator | head -30   # copy bootstrap capability
# Browser: http://127.0.0.1:8080/ → complete wizard
mynet admin bootstrap --server http://127.0.0.1:8080   # optional CLI check
```

Do **not** reset staging DB or restart production/staging services without operator approval.

## CLI

```bash
mynet admin bootstrap --server http://127.0.0.1:8080
```

Returns JSON with `bootstrap_token` and `bootstrap_expires_at` (loopback only).
