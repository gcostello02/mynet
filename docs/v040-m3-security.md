# M3 dashboard — security notes

## Trust model

- Administrator and device traffic terminate TLS at your reverse proxy; the Go process listens on plain HTTP locally.
- The dashboard is **same-origin** with `/v1/*`. Do not host the API and UI on different registrable domains without adding explicit CORS (not supported in M3).
- Set `MYNET_SECURE_COOKIES=1` on `mynet-server` when users reach the coordinator over HTTPS so session cookies are `Secure`. Staging compose sets this by default; production operators must set the same behind TLS.

Browser login responses **omit** `session_token` in JSON; only the `mynet` CLI sends `X-Mynet-Admin-Client: cli` to receive a bearer token. Cookie-authenticated `POST`/`PATCH`/`DELETE`/`PUT` require `X-Mynet-CSRF` and a matching `Origin`/`Referer` host when those headers are present.

## Browser session

| Mechanism | Detail |
|-----------|--------|
| Session | `mynet_admin_session` HttpOnly cookie (12h) |
| CSRF | `mynet_csrf` readable cookie + `X-Mynet-CSRF` header on cookie-authenticated mutating requests |
| CLI | Unchanged: Bearer `session_token` from login JSON, no CSRF |
| Storage | No admin tokens or invitation strings in `localStorage` (theme preference only) |

## Response headers (dashboard static)

- `Content-Security-Policy`: `default-src 'self'` (no third-party scripts/fonts)
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Referrer-Policy: strict-origin-when-cross-origin`

## What the UI does not claim

- **Signaling online** reflects WebSocket presence, not WireGuard handshake or tunnel health.
- Coordinator private keys and device private keys are never sent to the browser.

## Coordinator restart

Admin sessions are in-memory. After restart, the dashboard shows login again; this is expected until persistent sessions are implemented.
