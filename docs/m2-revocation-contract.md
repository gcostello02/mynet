# M2 client revocation reconciliation contract (draft)

Coordinator M1 enforces revocation on the **control plane** immediately. Clients must reconcile the **data plane** locally.

## Coordinator guarantees (immediate)

| Action | Effect |
|--------|--------|
| Admin revoke | `revoked=1`, `auth_epoch++`, removed from active peer maps |
| `POST /v1/auth` | Fails for revoked / unknown device |
| Bearer + WS | `verifyDeviceBearer` fails; hub disconnect on revoke |
| `POST /v1/sessions` | Fails (device not in `GetDevice`) |
| `GET /v1/peers` / `GET /v1/vpn/config` | Revoked devices omitted from peer lists |

## Client responsibilities (M2)

1. On auth failure with “unknown device” / “token revoked”, clear `session.json` and stop signaling.
2. Periodically refresh `GET /v1/vpn/config` (existing daemon refresh interval or new explicit interval).
3. Remove WireGuard peers not present in latest config; do not trust stale local peer cache after refresh.
4. Do not treat `signaling_online` from admin API as VPN reachability.

## Out of scope for M1

- Pushing revoke events to connected clients (no server-side WG reconfigure).
- Forcing UDP tunnel teardown without client config refresh.
