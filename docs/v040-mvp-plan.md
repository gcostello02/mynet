# MyNet V0.4 — MVP plan & roadmap

**Checkpoint:** architecture and plan only — no implementation until review. **Do not modify live coordinator** (`mesh.example.com`) or production devices during planning.

---

## Phase 2 — V0.4 MVP scope

### In scope (MVP)

| Capability | Notes |
|------------|--------|
| One coordinator per network | Extend current `mynet-server`; single SQLite DB |
| Administrator identity | Password or passkey-backed **admin** separate from device tokens |
| Coordinator bootstrap | Guided first-run: secrets, listen URL, data path, optional TURN hints |
| Device invitations | Short-lived, one-time or limited-use enrollment tokens (replace shared secret for day-to-day) |
| ≥3 enrolled devices | Reuse multi-peer VPN + signaling |
| Automatic VPN IPv4 | Existing allocator |
| Peer discovery & WG config | Existing `/v1/peers`, `/v1/vpn/config` |
| Device list, rename, revoke | New **admin** REST; enforce on auth + VPN register |
| Lightweight web dashboard | Static assets + admin API |
| macOS/Linux supervised clients | V0.3.0 paths; enroll via CLI or installer script |
| Health & diagnostics | Dashboard + `mynet status` / `mynet diagnose`; distinguish WS online vs VPN ping |

### Out of scope (post–MVP)

| Item | Rationale |
|------|-----------|
| Windows client | Later port of service + TUN stack |
| Hosted SaaS / multi-tenant cloud | Conflicts with self-hosted vision |
| Automatic DNS / MagicDNS | Nice-to-have |
| Per-device RBAC | Admin vs device is enough for MVP |
| Pending approval queue (optional) | MVP: invitation = pre-approval; **later:** enroll → admin approves |
| Key rotation UI | Design in security doc; implement after revoke stable |
| Full Docker-first coordinator | Ship native systemd + docs first; Compose as secondary |
| In-place migration of live mesh secrets | Operator-driven; no automated touch of production |

---

## Phase 3 — Coordinator setup (recommendation)

### Options compared

| Path | MVP fit | Pros | Cons |
|------|---------|------|------|
| **Native Linux binary + systemd** | **Primary** | Matches mini-PC production; clear TURN UDP story; no container NAT surprises | Operator writes unit + env |
| Docker Compose | Secondary | Reproducible; good for homelab | TURN UDP relay still needs host ports; TLS often external |
| Existing server + reverse proxy | Common | Reuse Caddy/nginx + Let’s Encrypt | User must wire WSS; TURN separate |
| VPS | Recommended in docs | Reachable from cellular/CGNAT | Cost |

**Primary MVP installation:** **Linux native `mynet-server` + systemd**, TLS via **reverse proxy** (Caddy or nginx) on user-chosen hostname. Document **home mini PC + port forward** as alternate (`docs/deployment-options.md`).

### Guided bootstrap flow (design)

1. **First start detection** — empty DB + no admin record → “setup mode” (**localhost only**, or `MYNET_SETUP_TOKEN` + `X-Mynet-Setup-Token` for explicit remote bootstrap). **M1 implements this minimal bootstrap; M4 adds guided installer/wizard.**
2. **Admin creates account** — username + password (argon2 hash in DB) or Ed25519 admin key + challenge.
3. **Network metadata** — display name, public `server_url` (for clients), VPN CIDR confirm.
4. **Secrets** — generate `MYNET_SERVER_SECRET`, rotate enrollment to **invitation-only** after bootstrap.
5. **Optional TURN** — wizard collects `MYNET_TURN_URLS` + secret; links to coturn doc.
6. **Health check** — `GET /healthz` + admin `GET /v1/admin/status`.
7. **First invitation** — admin UI/CLI mints token; operator runs `mynet join` on second device.

Constraints honored:

- No hardcoded domain — `server_url` stored in DB/config.
- HTTPS/WSS via proxy; credentials not in query strings for admin (HttpOnly cookie or `Authorization`).
- TURN documented separately from HTTPS tunnel (Cloudflare Tunnel ≠ TURN).

---

## Phase 5 — Dashboard architecture

### Stack recommendation

- **Go `embed` FS** or `internal/control/server/static/` for HTML/CSS/JS (no React requirement).
- **HTMX or vanilla fetch** for device list and invitations — minimal build pipeline.
- **Same origin** as API to simplify cookies; CORS only if split later.

### Pages

| Page | Data sources |
|------|----------------|
| Overview | Device count, online (WS), last enroll, server version |
| Devices | Admin list devices; rename; revoke; show VPN IP, WG pubkey fingerprint |
| Invitations | Create/revoke tokens; expiry; uses remaining |
| Network settings | Public URL, VPN CIDR (read-only post-bootstrap), STUN/TURN display |
| Connection health | Per-device: signaling online; optional last diagnostic from client-reported heartbeat (later) |
| Administration | Change password, rotate server secrets (advanced), export backup hint |

**Presence vs data path:** `online` from hub ≠ VPN reachable. Dashboard labels **“Signaling connected”** vs **“VPN last seen”** (from optional client metric or operator-run `mynet ping`).

### Proposed admin REST (draft contract)

Prefix: `/v1/admin` — all require admin session (cookie or bearer).

| Method | Path | Body / response |
|--------|------|-----------------|
| POST | `/v1/admin/setup` | First-run only: `{ password, server_url, network_name }` |
| POST | `/v1/admin/login` | `{ username, password }` → session |
| POST | `/v1/admin/logout` | |
| GET | `/v1/admin/devices` | `[{ id, name, vpn_ipv4, wg_pubkey, revoked, signaling_online, created_at }]` |
| PATCH | `/v1/admin/devices/{id}` | `{ name }` |
| POST | `/v1/admin/devices/{id}/revoke` | |
| GET | `/v1/admin/invitations` | List tokens (metadata only) |
| POST | `/v1/admin/invitations` | `{ label?, expires_in, max_uses }` → `{ token, enroll_url }` |
| DELETE | `/v1/admin/invitations/{id}` | |
| GET | `/v1/admin/network` | CIDR, server_url, turn_configured |

**Device enroll change:** `POST /v1/enroll` accepts `enrollment_token` (JWT or random id) instead of global secret when invitations enabled.

**CLI parity:** `mynet devices list|remove`, `mynet network invite`, `mynet status` call same admin API (admin login subcommand stores admin session separately from device `session.json`).

### Data model extensions (SQLite)

```text
admins(id, username, password_hash, created_at)
invitations(id, token_hash, label, expires_at, max_uses, uses, created_by, revoked_at)
network_meta(key, value)  -- server_url, display_name, setup_complete
devices(...)              -- existing + optional last_seen_at, renamed_at
```

---

## Phase 6 — Installer & CLI (design)

### Command sketch (align with existing `mynet` verbs)

| Command | Purpose |
|---------|---------|
| `mynet network create` | Local wizard: write config, print coordinator install steps |
| `mynet coordinator init` | On server host: bootstrap DB + systemd unit template |
| `mynet join <invitation>` | `init` + enroll with token + `login` + optional `service install` |
| `mynet devices list` | Admin API |
| `mynet devices remove <name\|id>` | Revoke |
| `mynet status` | Local daemon + signaling + VPN summary |
| `mynet diagnose [--peer]` | Existing |

Installers (shell):

- Verify release checksums (sig or SHA256 from GitHub releases).
- Install libexec binary; **do not** overwrite `identity.json` / `wg.json` without `--force`.
- Run `service install` only when no competing manual daemon (`ShouldSpawnDetachedDaemon` checks).
- Upgrade: reuse `service upgrade --stage-only --commit`.

---

## Phase 7 — Testing plan

### Automated (CI)

| Suite | Coverage |
|-------|----------|
| `go test ./...` | Existing unit + e2e (`runtime_e2e_test`, server tests) |
| New: coordinator + 3 clients in-process | httptest server + 3 fake identities; enroll, auth, vpn config, revoke → auth fail |
| Invitation token | Single-use, expiry, wrong token |
| IP allocation | No double-assign; revoke does not recycle IP in MVP (document behavior) |

### Integration (manual / staging lab)

- Fresh VPS or VM coordinator — **not** production mesh.
- Three hosts: enroll, `up`, ping all pairs.
- Scenarios: peer restart, coordinator restart, network flap (netwatch), revoked device reconnect attempt, mixed client versions (n-1 binary).

Simulations cannot replace UDP/NAT behavior — mark **real multi-host** as release gate for V0.4.0.

---

## Phase 8 — Implementation milestones

### M0 — Docs & API design sign-off (current)

- [x] Architecture audit
- [ ] Review approved (this checkpoint)

### M1 — Admin & invitations (coordinator) — **implemented (review checkpoint)**

- [x] DB migrations: admins, invitations, network_meta, device `auth_epoch`
- [x] Local/setup-token bootstrap + admin login (CSRF on cookie mutations)
- [x] `POST /v1/enroll` invitation path + `coordinator_fingerprint`
- [x] Legacy enrollment when `MYNET_ENROLLMENT_SECRET` + persisted `legacy_enrollment_enabled`
- [x] Admin revoke/rename/list + invitation CRUD
- [x] Device token epoch + signaling disconnect on revoke
- [x] `GET /v1/network/public`
- [ ] Dashboard (M3), `mynet join` (M2), coordinator wizard (M4)

**Depends on:** M0 approval  
**Risk:** production cutover still operator-driven; legacy auto-enabled when enroll secret present on existing DBs

### M2 — Client & CLI

- `mynet join`, admin device commands
- Separate admin credentials file e.g. `~/.mynet/admin-session.json` (optional)
- Revoked device handling on auth (clear message)

**Depends on:** M1

### M3 — Dashboard

- Embedded UI + M1 APIs
- Overview + devices + invitations

**Depends on:** M1

### M4 — Coordinator bootstrap & packaging

- `mynet coordinator init` or `mynet-server --setup`
- systemd unit template in `deploy/`
- Update deployment docs (not live mesh)

**Depends on:** M1

### M5 — Installers & release

- Release artifacts macOS/Linux
- Installer scripts + checksums
- Staging acceptance (3 devices)

**Depends on:** M2, M3, M4

### Rollback strategy

- Coordinator: DB backup before migration; keep previous binary; env flag to allow legacy enrollment
- Clients: `service upgrade` swap back to staged previous binary; identity unchanged

### Acceptance criteria (V0.4.0)

1. New network on staging coordinator with admin bootstrap.
2. Three devices enrolled via invitations only.
3. Admin revokes one device; it cannot auth within 60s.
4. Dashboard lists devices and signaling online state.
5. macOS + Linux supervised install from release tarball without manual JSON editing.
6. Production mesh unchanged unless operator opts in.

---

## Open questions (for review)

1. Admin auth: password-only MVP vs WebAuthn passkey in V0.4?
2. Recycle VPN IPs after revoke or reserve forever?
3. Should enrolled devices read admin APIs or strictly admin-only credentials?
4. Coordinator setup: bind setup to localhost + SSH tunnel vs one-time setup token on public URL?
5. Client-reported VPN health to server — privacy vs dashboard usefulness?
