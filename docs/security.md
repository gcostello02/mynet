# Security model

## Trust boundaries

| Party | Trusted for | Not trusted for |
|-------|-------------|-----------------|
| **Coordinator** (yours) | Enrollment, address allocation, distributing peers' WireGuard public keys, signaling | Reading tunnel traffic (it never has WireGuard private keys and never carries VPN packets) |
| **Reverse proxy / TLS** (yours) | Authenticating the hostname | n/a |
| **STUN/TURN servers** | NAT traversal, relaying already-encrypted WireGuard packets | Confidentiality of traffic (it's WireGuard-encrypted end to end) |

A compromised coordinator could publish rogue WireGuard keys for peers or deny service. It cannot decrypt traffic between peers that already hold each other's real keys.

## Secrets and where they live

| Secret | Location | Notes |
|--------|----------|-------|
| Device Ed25519 private key | `~/.mynet/identity.json` (0600) | Never leaves the device |
| WireGuard private key | `~/.mynet/wg.json` (0600) | Never sent to the coordinator |
| Device bearer token | `~/.mynet/session.json` | 24 h TTL, renewed automatically |
| Admin session (CLI) | `~/.mynet/admin-session.json` | Separate from device credentials |
| `MYNET_SERVER_SECRET` | Coordinator `.env` | HMAC key for bearer tokens; at least 32 bytes; never commit |
| Coordinator Ed25519 key | `coordinator.ed25519` (0600) beside the DB | Back up together with the DB; the coordinator refuses to start if it's missing or corrupt |
| Admin password | SQLite, argon2id hash | Minimum 12 characters; login attempts are rate-limited |
| Invitation token | Shown once at creation | 256-bit random; only a hash is stored |
| `MYNET_TURN_SECRET` | Coordinator and coturn | Shared secret for time-limited TURN credentials |

## Enrolling a device

- **Invitations are one-time and expiring.** An invitation can be limited to a number of uses, and a failed enrollment does not consume a use. Admins can revoke an unused invitation.
- **The invitation pins the coordinator.** It contains the coordinator's Ed25519 fingerprint. `mynet join` requires an HTTPS URL, rejects redirects to another host, checks `GET /v1/network/public` against the fingerprint, and verifies a signed proof (`POST /v1/network/proof`, domain `mynet-coordinator-proof-v2`, with a random client challenge). TLS authenticates the hostname; the fingerprint authenticates the network, so a valid TLS certificate for the wrong coordinator is rejected.
- **Deliver invitations out of band.** If an attacker controls both the invitation and the network path, pinning can't help. Treat invitations like passwords.
- **Enrollment is signed.** The device signs `mynet-enroll-v1 | device_id | name | timestamp` with its Ed25519 key. Authentication (`mynet-auth-v1`) and WireGuard registration (`mynet-vpn-register-v1`) are signed likewise, and timestamps must be within a small skew window.
- **Addresses are server-allocated.** Clients can't choose an IP, and a WireGuard public key can't be bound to two devices.
- **Legacy shared-secret enrollment** (`MYNET_ENROLLMENT_SECRET`) is off for new networks. It's only auto-enabled when migrating a database that already has devices, and it can be toggled with `PUT /v1/admin/network/legacy-enrollment`.

## Revoking a device

1. An admin revokes the device (dashboard or `mynet admin devices revoke <id>`).
2. Its bearer tokens stop working immediately (a per-device auth epoch is bumped) and its signaling WebSocket is dropped.
3. Peers drop its WireGuard key on their next config refresh (every 45 s while online). A revoked device can't re-enroll.

## First-run setup

- A network is initialized once: setup fails once it's complete, or if devices are already enrolled.
- Setup from the host itself (loopback) is allowed. Setup from elsewhere needs a one-time bootstrap token, which is printed in the coordinator log, valid for 2 hours, cleared after use, and obtainable only over loopback (`mynet admin bootstrap`).
- Loopback is decided from the socket's remote address, never from `X-Forwarded-For`.

## Admin dashboard

- The UI is same-origin with the API. Don't host them on different domains.
- Browser sessions use an HttpOnly, `SameSite=Strict` cookie (12 h). Set `MYNET_SECURE_COOKIES=true` behind HTTPS.
- Mutating requests need a CSRF header matching the `mynet_csrf` cookie, and a matching `Origin`/`Referer` host when present.
- Browser login responses omit the session token. Only the CLI (header `X-Mynet-Admin-Client: cli`) receives a bearer token.
- No admin tokens or invitation strings go in `localStorage`; the invitation is shown once in memory.
- Responses carry a strict CSP (`default-src 'self'`), `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY` and a strict `Referrer-Policy`.
- Rate limits key on the connection's `RemoteAddr`, not forwarded headers.
- "Signaling online" in the UI means a device holds a WebSocket. It doesn't prove a working tunnel.
- Changing the admin password (`mynet admin password change`) invalidates every admin session.

## Threats and mitigations

| Threat | Mitigation |
|--------|------------|
| Stolen invitation | Short lifetime, limited uses, HTTPS only, revocable |
| Impersonated coordinator (DNS hijack, captive portal) | TLS plus fingerprint pinning and signed proof |
| Stolen device | Revoke it; tokens die immediately and peers drop its key |
| Replayed enroll/auth requests | Signed timestamps with a skew window; single-use invitations |
| Two devices claiming one identity or key | Server rejects mismatching keys |
| Database theft | File permissions and disk encryption; no plaintext passwords or invitation tokens stored |
| Compromised coordinator | Can't read traffic; can misdirect or deny. Run it on a host you control |

## Known limitations

- Rotating the coordinator key means re-running setup and re-inviting devices (new fingerprint).
- Admin sessions don't survive a coordinator restart.
- There's no automated WireGuard key rotation; re-register by re-joining.
- If the coordinator is compromised, peers have no independent way to detect rogue keys (key-change monitoring is future work).

## Reporting issues

Please report vulnerabilities privately to the repository owner rather than in a public issue.
