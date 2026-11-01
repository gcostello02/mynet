# M3 browser security audit (2026-03)

## Findings (by severity)

### Medium (fixed in this milestone)

| ID | Issue | Fix |
|----|--------|-----|
| M1 | Login JSON exposed `session_token` to any client (XSS could exfiltrate alongside cookies) | Token only returned when `X-Mynet-Admin-Client: cli` (mynet CLI) |
| M2 | Logout accepted session cookie without CSRF (logout CSRF with `SameSite=Strict` was already unlikely) | Logout requires same CSRF + origin rules as other mutations |
| M3 | Cookie-authenticated mutations had CSRF but no `Origin`/`Referer` check | Reject mutations when `Origin`/`Referer` host ≠ request `Host` (browser clients); CLI/tests unchanged when both absent |
| M4 | Admin API responses lacked `Cache-Control: no-store` | Middleware on `/v1/admin/*` |
| M5 | Staging coordinator did not set `MYNET_SECURE_COOKIES` | `MYNET_SECURE_COOKIES=1` in staging compose |
| M6 | CSP allowed implicit script/style from default-src only | Explicit `script-src 'self'` … `font-src 'self'` on dashboard static |

### Low / informational

| ID | Issue | Status |
|----|--------|--------|
| L1 | Admin sessions in-memory; coordinator restart forces re-login | Documented; expected until DB-backed sessions |
| L2 | `mynet_csrf` cookie readable by JS (double-submit pattern) | Accepted; mitigated by strict CSP + no token in localStorage |
| L3 | Login CSRF could bind victim browser to attacker-controlled admin session | Low impact for admin UI; `SameSite=Strict` limits cross-site cookie use |
| L4 | Rate limits use `RemoteAddr`, not `X-Forwarded-For` | By design (see v040-security-model) |
| L5 | Enrollment token in create-invitation JSON still visible to browser JS once | By design; shown once, not stored in localStorage |

### Not issues

- Bearer CLI auth skips CSRF (documented automation path).
- Coordinator does not trust `X-Forwarded-*` for security decisions.
- Device/admin authorization split unchanged.

## Fixes applied

See commit on `feature/v040-m3-dashboard`: `admin_browser_security.go`, login/logout handlers, CLI login header, CSP/cache headers, staging env, tests in `admin_browser_security_test.go`.

## Remaining risks

- **XSS in dashboard** remains the primary cookie/session threat; CSP reduces but does not eliminate supply-chain or future UI bugs.
- **No HSTS** at application layer (rely on reverse proxy).
- **Session persistence** across restart not implemented.
- **WebSocket `CheckOrigin: true`** is device signaling, not admin; still permissive for ICE.

## Staging validation (operator)

1. Rebuild coordinator image after pulling this commit (`docker compose build coordinator` — operator approval).
2. Open staging HTTPS URL; confirm login sets cookies with **Secure** flag (browser devtools → Application → Cookies).
3. Confirm dashboard works: devices, invite create, revoke.
4. Confirm `mynet admin login` still works against staging (CLI header receives token).
5. Optional: `curl -i https://…/v1/admin/status` without cookies → `401` and `Cache-Control: no-store`.

Do not expose staging to the public internet or reset databases without explicit approval.
