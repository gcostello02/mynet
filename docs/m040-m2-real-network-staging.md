# M2 real-network staging harness

Isolated **self-hosted** validation for WireGuard, ICE, enrollment, recovery, and revocation. **Never** targets `mesh.example.com`, production `devices.db`, or live client state.

## Prerequisites

| Requirement | Notes |
|-------------|--------|
| Linux host or **Docker Desktop** (Linux VM backend) | Real TUN/WG needs Linux; macOS host does not provide `ip netns` |
| Docker Engine 24+ | `docker compose` v2 |
| `openssl`, `jq`, `curl` | CA generation and bootstrap |
| **Privileges (reviewed by operator)** | Client containers use `privileged: true`, `CAP_NET_ADMIN`, `/dev/net/tun` |
| Disk | ~2 GB for images + volumes |

**Not required:** Tailscale, Cloudflare Tunnel, production mini PC access, root on the host (only inside containers).

### Mini PC / production

Repository docs (`deploy/minipc-server-start.md`) describe the **production** coordinator on the home mini PC (`~/mynet`, `mesh.example.com`, VPN pool `172.31.240.0/20`). **This harness does not SSH to or modify that host.** No remote inspection was performed in automation; treat the mini PC as production unless you verify otherwise locally.

## Isolation model

| Boundary | Staging value | Production (untouched) |
|----------|---------------|-------------------------|
| Docker bridge | `10.200.0.0/24` | LAN / Tailscale |
| VPN pool | `10.88.0.0/24` (`MYNET_VPN_CIDR`) | `172.31.240.0/20` |
| Coordinator URL | `https://staging-coord.mynet.test` (port `127.0.0.1:18443` on host) | `https://mesh.example.com` |
| TLS trust | Local staging CA (`deploy/staging/certs/ca.crt`) | Public PKI / Cloudflare |
| State | Compose volumes `client-*-state`, `coord-data` | `~/.mynet`, `~/mynet/devices.db` |

Host default routes, `systemd`/`launchd`, and production WireGuard interfaces are **not** modified. Staging binds HTTPS to **localhost only** (`127.0.0.1:18443`).

## Setup

```bash
cd deploy/staging
chmod +x scripts/*.sh entrypoint-*.sh
./scripts/gen-ca.sh
export MYNET_SERVER_SECRET="$(openssl rand -base64 32 | tr -d '\n')"
docker compose build
docker compose up -d coordinator proxy
./scripts/bootstrap-coordinator.sh
./scripts/enroll-three.sh
```

## Test execution (privileged clients)

```bash
export MYNET_STAGING_I_UNDERSTAND=1
./scripts/run-acceptance.sh
```

Or stepwise after enrollment:

```bash
docker compose --profile clients up -d
./scripts/data-plane.sh
./scripts/recovery.sh
./scripts/revoke.sh
```

Artifacts: `deploy/staging/artifacts/*.txt`, `*-results.txt`.

## Verification checklist

- [ ] Three distinct `device_id` and `vpn_ipv4` in `wg.json` per client
- [ ] ICMP A↔B, A↔C, B↔C over VPN addresses
- [ ] `mynet ping --peer` for each pair
- [ ] `mynet diagnose` shows ICE path (P2P or relay) — **real** negotiation, not httptest
- [ ] Coordinator restart: clients keep identity; signaling/VPN recover
- [ ] Revoke client-c: auth fails; peer removed from others; measured delay logged in `revoke-results.txt`
- [ ] Authorized pair A↔B still pings after revoke

## Partial-run recovery

If acceptance stops after coordinator bootstrap (e.g. invitation step failed):

1. **Resume** (same coordinator DB and secret):
   ```bash
   cd deploy/staging
   export COMPOSE_PROJECT_NAME=mynet-m2-staging
   # secrets/server.secret must match the running coord-data volume
   docker compose up -d coordinator proxy
   ./scripts/enroll-three.sh
   ```
2. **Stop and clear local admin files** (coordinator DB **kept**):
   ```bash
   MYNET_STAGING_RESET=1 ./scripts/reset-staging.sh
   ```
3. **Fresh coordinator database** (only project `mynet-m2-staging` volumes):
   ```bash
   MYNET_STAGING_FRESH=1 ./scripts/fresh-staging.sh
   ```

`bootstrap-coordinator.sh` skips setup when `/v1/network/public` already responds. `enroll-three.sh` skips existing `artifacts/inv-*.txt` and reuses `admin-state/admin-session.json`.

Admin commands use `docker compose run admin-toolbox` with entrypoint `/bin/bash` and `-lc` only (no nested `bash` argv). Smoke test: `./scripts/verify-admin-toolbox.sh`.

Secrets: `secrets/server.secret` (mode 600, gitignored). File takes precedence over `MYNET_SERVER_SECRET` in the shell unless `MYNET_STAGING_ALLOW_ENV_SECRET=1`. Removed by `fresh-staging.sh`.

## Cleanup

```bash
./scripts/teardown.sh
MYNET_STAGING_PURGE_VOLUMES=1 ./scripts/teardown.sh
MYNET_STAGING_PURGE_ARTIFACTS=1 ./scripts/teardown.sh
```

Partial failure: see `artifacts/last-error.txt` after a failed `run-acceptance.sh`.

## Known limitations

- **Simulated:** None for ICE/WG inside client containers once `mynet up` runs; coordinator/signaling are real binaries.
- **Not simulated:** Enrollment proof v2 and join against HTTPS with staging CA (not `AllowInsecureHTTPForTest`).
- **STUN:** Uses public `stun.l.google.com` for reflexive candidates; optional TURN not provisioned by default.
- **Revocation delay:** Daemon polls VPN config every 45s; measured delay may be **less** if auth refresh or ICE events occur earlier.
- **Offline peers:** Cannot shrink WG state without coordinator reachability (see `docs/m2-revocation-contract.md`).
- **macOS host:** Run acceptance via Docker; do not expect native `mynet up` on Darwin for this harness.

## Go unit tests

```bash
go test ./... -count=1
go test -race ./... -count=1
```

Linux-only hook (reads artifacts after a manual run):

```bash
MYNET_STAGING_ROOT=deploy/staging go test ./internal/staging/ -count=1
```
