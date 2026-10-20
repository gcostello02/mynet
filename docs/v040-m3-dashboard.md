# V0.4 M3 — Self-hosted management dashboard

## Architecture

The coordinator (`mynet-server`) continues to expose a single HTTP listener. API routes (`/v1/*`, `/healthz`) register on the Go 1.22 `ServeMux` first. The embedded SPA is served only for `GET` requests that do not match API paths:

- Static hashed assets under `/assets/*`
- `index.html` for dashboard client routes (`/`, `/login`, `/devices`, `/invitations`, `/settings`)

No separate frontend container or CDN. Assets are produced by `web/admin` (Vite + React + TypeScript + Tailwind) and embedded with `go:embed` in `internal/dashboard`.

## Existing API inventory (M3 uses)

| Method | Path | Auth | Dashboard use |
|--------|------|------|----------------|
| GET | `/healthz` | none | Coordinator liveness |
| GET | `/v1/network/public` | none | Network name, fingerprint (settings + login banner) |
| POST | `/v1/admin/login` | none | Login; sets HttpOnly session + CSRF cookies |
| POST | `/v1/admin/logout` | session | Logout |
| GET | `/v1/admin/status` | session | Overview stats |
| GET | `/v1/admin/devices` | session | Devices list/detail |
| POST | `/v1/admin/devices/{id}/revoke` | session + CSRF | Revoke |
| GET | `/v1/admin/invitations` | session | Invitation list |
| POST | `/v1/admin/invitations` | session + CSRF | Create (one-time token) |
| DELETE | `/v1/admin/invitations/{id}` | session + CSRF | Revoke invitation |
| PUT | `/v1/admin/network/legacy-enrollment` | session + CSRF | Read-only display of flag from status |

**Not implemented (gaps):** `GET /v1/admin/network` (use public + status). Device WireGuard handshake state is not exposed — UI shows **signaling online** only, not “VPN connected”. Admin sessions are in-memory (restart invalidates browser sessions). No server version endpoint yet — build embeds `internal/version.Version`.

## Security design

### Browser authentication

- Prefer **cookie session** (`mynet_admin_session`, HttpOnly, `SameSite=Strict`, `Secure` when `MYNET_SECURE_COOKIES=1`).
- CSRF: double-submit via `mynet_csrf` cookie + `X-Mynet-CSRF` on mutating `/v1/admin/*` requests when using cookies.
- CLI continues to use `Authorization: Bearer <session_token>` without CSRF (unchanged).

### Frontend

- No `localStorage` for session or invitation secrets.
- Enrollment token shown once in a modal after create; not persisted.
- React text rendering only (no `dangerouslySetInnerHTML`).
- `fetch(..., { credentials: 'include' })` same-origin only.

### Coordinator static responses

- `Content-Security-Policy`: default-src 'self'; script/style from self; no external origins.
- `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: strict-origin-when-cross-origin`.
- Dashboard HTML not served for `/v1/*` or `/healthz`.

### Reverse proxy trust

TLS termination at Caddy/nginx is assumed. Set `MYNET_SECURE_COOKIES=1` so cookies are marked Secure. The coordinator does not validate `X-Forwarded-*` for host binding; deploy the dashboard on the same public host as the API (same origin). Do not expose the plain HTTP listener directly to the Internet without a proxy.

## Build

```bash
cd web/admin && npm ci && npm run build   # writes internal/dashboard/dist
go build -o bin/mynet-server ./cmd/mynet-server
```

CI runs the frontend build before Go tests. `make build-dashboard` wraps the npm step.

## Local preview

```bash
cd web/admin && npm run dev   # proxies /v1 to coordinator (see vite.config.ts)
# or build + run mynet-server with MYNET_DATA_PATH set
```

## Remaining before M3 acceptance

- [ ] Operator sign-off on staging with real HTTPS + `MYNET_SECURE_COOKIES`
- [ ] Device rename UI (API exists; optional for M3)
- [ ] Invitation delete/revoke in UI
- [ ] Persist admin sessions across coordinator restart (schema exists, unused)
- [ ] E2E browser tests in CI (Playwright) — optional follow-up
- [x] Setup wizard UI + one-time bootstrap capability (`GET /v1/setup/status`, `POST /v1/admin/setup`) — see [v040-m3-first-run-setup.md](v040-m3-first-run-setup.md)
