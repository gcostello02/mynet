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

## Client responsibilities (M2 — implemented)

1. On auth failure (401/403) during VPN config refresh, clear `session.json` via `Runtime.ClearDeviceSession()`.
2. Daemon `startVPNReconcile` polls `GET /v1/vpn/config` every **45s** (`VPNReconcileInterval`) while VPN is up. **Maximum expected revocation delay on connected peers: ~45s** (plus auth refresh latency), bounded by the reconcile ticker.
3. Peers missing from config: `wgengine.RemovePeer`, `Runtime.DropPeerForRevocation` (ICE + link cache).
4. New peers in config are added to WireGuard when they appear.
5. Do not treat `signaling_online` as VPN reachability.

**Offline limitation:** peers without connectivity to the coordinator keep stale WG state until refresh succeeds.

## Out of scope for M1

- Pushing revoke events to connected clients (no server-side WG reconfigure).
- Forcing UDP tunnel teardown without client config refresh.
