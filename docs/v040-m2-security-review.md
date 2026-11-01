# V0.4 M2 security review and staging acceptance

**Branch:** `feature/v040-m2-secure-join`  
**Scope:** M2 secure join, admin CLI, VPN reconciliation. Production V0.3 untouched.

## 1. Coordinator proof verification

| Requirement | Status |
|-------------|--------|
| Challenge-bound v2 proof for join | **Fixed:** `join.VerifyCoordinator` requires `proof_version >= 2`; no v1 fallback on join |
| No silent downgrade on invalid v2 | **Fixed:** invalid signature/metadata rejected; v1 responses return `ErrProofDowngrade` |
| Network ID, fingerprint, pinned pubkey, challenge | Verified before signature check |
| Redirect host/scheme | `control.NewSecureAPIClient` blocks cross-host and non-HTTPS redirects |
| TLS validation | Default `http.Client` (no `InsecureSkipVerify` in tree) |
| Tests: forged, mismatch, downgrade, redirect | `internal/join/verify_test.go`, `internal/control/httpclient_test.go` |

**Legacy:** v1 proof remains on server for non-join callers; join path is v2-only.

## 2. Join lifecycle

| Scenario | Coverage |
|----------|----------|
| Fresh join / three devices | `TestJoinThreeDevices`, coordinator tests |
| 409 without implying success | Enroll 409 only continues if `Auth` succeeds for this device |
| Existing identity / `--force` | CLI + `TestJoinRejectsExistingIdentityWithoutForce`, `TestJoinForceReplacesIdentity` |
| Force clears WG keys | `wgkeys.Clear` on `--force` |
| Daemon running during join | Blocked in `mynet join` via `daemon.DaemonReachable` |
| HTTPS required | Invitation validate + `NewSecureAPIClient` |
| Recovery hint | `client.JoinRecoveryHint` on enroll/auth/VPN failures |
| Interrupted join (identity before enroll) | **Gap:** no progress file; user must re-join or `mynet login` if enrolled |

Invitations are not echoed in structured errors from client helpers.

## 3. Peer reconciliation

- Failed `VPNConfig` fetch does **not** remove peers (unchanged).
- Peer removal only after successful config fetch.
- `reconcileMu` prevents overlapping reconcile runs.
- Malformed peer entries skipped (empty name/key/IP, bad key length).
- Max revocation delay: **~45s** (`VPNReconcileInterval`); documented in `docs/m2-revocation-contract.md`.
- **Gap:** no live ICE/WG race harness in CI; unit mutex test only.

## 4. Administrator CLI

- `admin-session.json` written `0600`, state dir `0700`.
- `mynet admin logout` clears session file.
- Admin bearer passed via `APIClient.Token` header (not argv); passwords read from stdin/stderr prompt only.
- Expired session: server returns 401; user must re-login (no silent refresh).

## 5. Staging topology (isolated)

| Component | Isolation |
|-----------|-----------|
| Coordinator | Temp SQLite + fresh `MYNET_SERVER_SECRET` |
| Clients | Separate `MYNET_STATE_DIR` per device |
| Production | **Not used** (`mesh.example.com`, Cloudflare tunnel, prod identities) |

Automated acceptance: `go test ./...` with in-process `httptest` coordinator (HTTP allowed only via test flags).  
Manual HTTPS staging: `scripts/m2-staging-acceptance.sh` + local TLS proxy (mkcert/openssl).

## 6. Three-device enrollment (automated)

`TestJoinThreeDevices` + lifecycle tests: unique device IDs, enroll, auth, VPN register against in-memory coordinator.

**Real-network harness:** `deploy/staging/` + `docs/m040-m2-real-network-staging.md` (Docker, isolated VPN `10.88.0.0/24`). Operator-run; requires Docker + privileged client containers.

## 7. Self-hosting

- Coordinator + SQLite + admin setup: no MyNet cloud account.
- STUN/TURN: optional external services; configurable ICE servers (not MyNet-operated infra).
- Join does not require a central device registry beyond the operator’s coordinator.

## 8. Test results

Run on review commit:

```bash
go test ./... -count=1
go test -race ./... -count=1
```

## 9. Remaining release blockers (M2 → M3)

1. End-to-end HTTPS staging with trusted CA (not httptest self-signed) on three hosts.
2. Live VPN data-plane + revocation timing validation (WG + ICE).
3. Join interrupt/resume with explicit progress state.
4. Deeper reconcile integration tests (mocked `VPNConfig` transitions + ICE recovery race).

## 10. M3 recommendation

**HOLD** until M1 merges and item (1–2) are validated on isolated staging VMs/containers. M2 control-plane security fixes and test coverage are sufficient for continued M2 hardening, not for M3 feature work.
