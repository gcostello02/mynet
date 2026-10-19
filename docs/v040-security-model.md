# MyNet V0.4 — Security model

Design for enrollment, administration, and threat handling. **Do not reuse production enrollment secrets or live mesh credentials in tests.**

---

## Phase 4 — Assets & trust boundaries

| Asset | Location | Must never |
|-------|----------|------------|
| Device Ed25519 private key | `~/.mynet/identity.json` | Leave device on enroll |
| WireGuard private key | `~/.mynet/wg.json` | Be sent to coordinator |
| Device bearer token | `~/.mynet/session.json` | Be logged or embedded in URLs long-term |
| Admin password / session | Server DB + HttpOnly cookie | Share with device clients |
| `MYNET_SERVER_SECRET` | Coordinator env | Be committed to git |
| Invitation token | QR/link one-time | Be reusable indefinitely |
| TURN shared secret | Coordinator + coturn | Be exposed to browsers |

**Trust:** Clients trust TLS to `server_url` and (V0.4) optional **coordinator identity** document published at `GET /v1/network/public` (server Ed25519 or TLS SPKI pin — TBD in implementation).

---

## Current primitives (reuse)

| Mechanism | Detail |
|-----------|--------|
| Enroll signature | `mynet-enroll-v1` + device_id + name + timestamp |
| Auth signature | `mynet-auth-v1` + device_id + timestamp |
| WG register signature | `mynet-vpn-register-v1` + device_id + wg_pubkey + timestamp |
| Bearer token | HMAC-SHA256 payload, server secret |
| Enrollment gate | Shared secret compared to bcrypt/argon hash |
| Revocation | `revoked` flag in SQLite; `GetDevice` fails |

---

## V0.4 additions

### Administrator authentication

- **Separate** from device bearer tokens.
- Password stored as **argon2id** hash; rate-limit login attempts.
- Session: random session id in HttpOnly, Secure, SameSite cookie; server-side session table or signed admin JWT with short TTL.
- Bootstrap: only when `setup_complete=false` and request from configured setup policy (localhost or setup token).

### Device invitations

| Property | Recommendation |
|----------|----------------|
| Format | 256-bit random, base64url; store **hash** only |
| Lifetime | Default 24h; max uses 1 for “join this laptop” |
| Binding | Optional: expected device name prefix, not key material |
| Revocation | Admin deletes invitation; unused tokens invalid |

**Enroll request:** `enrollment_token` + `coordinator_fingerprint` + signed identity fields. Server validates fingerprint and invitation **before** enroll; invitation use is **redeemed only after successful** `EnrollDevice` (failed enroll does not consume a use).

### Pending approval (post-MVP option)

MVP: **invitation = approval**. Later: enroll creates `pending` row until admin approves.

### Coordinator verification (client)

Threat: **impersonated coordinator** (DNS hijack, rogue AP captive portal).

Mitigations:

1. TLS with publicly trusted CA (operator responsibility).
2. V0.4 (M1): **`GET /v1/network/public`** publishes `coordinator_fingerprint` (SHA-256 of coordinator Ed25519 public key). Invitation API returns the same fingerprint. **Coordinator private key is generated at setup and not persisted** — M1 does not sign attestations; trust is TOFU via out-of-band fingerprint (invitation channel) compared to TLS fetch in M2.
3. MITM risk: if the client learns the fingerprint only from the same hostile HTTPS endpoint, substitution is still possible. **M2 must compare invitation fingerprint to an independently obtained value** (QR, admin console copy, etc.).
4. Later: optional signed network documents if coordinator private key is stored securely; optional `~/.mynet/trust.json` pin file.

### Address allocation

- Server-authoritative; client cannot pick arbitrary IP.
- Reject WG pubkey already bound to another device.
- On revoke: **MVP keep IP reserved** to avoid confusion; document; later policy for recycle.

### Device revocation

1. Admin `POST .../revoke` → `revoked=true`.
2. Existing bearer tokens: **`auth_epoch` in token** invalidated immediately on revoke (plus `GetDevice` failure).
3. Disconnect signaling: hub kick on revoke.
4. VPN: peers stop including revoked WG pubkey on next config fetch; local daemon refresh interval.

### Key rotation & recovery

| Scenario | MVP | Later |
|----------|-----|-------|
| Lost device | Admin revoke + new invitation | |
| Lost admin password | Offline recovery code at bootstrap | |
| Rotate WG keys | Re-register endpoint | Automated rotation |
| Rotate server secret | All clients re-auth | Dual-secret grace period |

### Separation of credentials

```text
Operator laptop (optional):
  ~/.mynet/admin-session.json   # management only

Every mesh device:
  identity.json + wg.json + session.json   # mesh participant only
```

CLI: `mynet admin login` vs `mynet login` (device).

---

## Threat matrix

| Threat | Mitigation |
|--------|------------|
| Stolen enrollment token | Short TTL, single use, HTTPS only, admin can revoke invitation |
| Stolen device session | TTL; revoke device; prefer header auth over WS query token long-term |
| Replay enroll/auth | Timestamp skew window (existing); nonce optional in V0.4.1 |
| Duplicate device_id | Server rejects pubkey mismatch on same id |
| Compromised device | Revoke; remove from VPN config |
| Replay signaling | Session bound to device_id + session_id in hub |
| MITM coordinator | TLS + TOFU fingerprint display |
| Insider with old enrollment secret | Disable legacy secret after bootstrap; rotate |
| SQLite theft | File permissions; host disk encryption; secrets not in DB plaintext |

---

## What stays on device (invariant)

- WireGuard private key generation **local** (`internal/identity` / wg keygen).
- Ed25519 device key **local** at `mynet init`.
- Private keys never in enrollment HTTP body except signatures.

---

## Testing & production isolation

- E2E tests use dedicated `MYNET_ENROLLMENT_SECRET` in temp dirs.
- Staging coordinator on separate hostname.
- **No changes** to `mesh.example.com` secrets or DB during V0.4 development.

---

## Compliance with V0.3 safety

Security work in V0.4 is **additive APIs and staging** until operator migrates.

**Legacy enrollment (M1):**

- Auto-enabled in SQLite only when the database **already contains enrolled devices** and `MYNET_ENROLLMENT_SECRET` is set (V0.3 migration).
- **New networks** after `POST /v1/admin/setup` default to `legacy_enrollment_enabled=false` (invitation-only).
- Admins may toggle via `PUT /v1/admin/network/legacy-enrollment` (requires coordinator env secret).
- Revoked devices **cannot** re-enroll via legacy or invitation (`errRevoked`).
- Admin setup is **blocked** when enrolled devices already exist (production mesh continues without admin bootstrap until a migration story).

## M1 audit notes (known limitations)

| Topic | Status |
|-------|--------|
| Admin sessions | In-memory only; lost on coordinator restart |
| Bearer admin auth | Skips CSRF (intended for future CLI); protect bearer like a secret |
| Rate limits | Use `RemoteAddr` only (not `X-Forwarded-For`) |
| WG data plane | Revoked peers removed from config on fetch; see `docs/m2-revocation-contract.md` |
| Coordinator key change | New setup generates new fingerprint; clients must re-pin |
