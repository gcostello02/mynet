# M2 staging — operator acceptance (revision ≥ `b1d98ca`)

Privileged Docker acceptance validates real ICE, WireGuard, recovery, revocation, and invitation replay. **Does not** touch production `mesh.example.com` or host networking.

## Preconditions

- Docker with `NET_ADMIN` / `/dev/net/tun` inside client containers
- `jq`, `openssl`, `curl` on the host
- Branch `feature/v040-m2-secure-join` at or after `b1d98ca`
- Operator sets `MYNET_STAGING_I_UNDERSTAND=1` for full automation

## Clean isolated run (new coordinator DB)

Only when you intend to wipe **staging** volumes:

```bash
cd deploy/staging
export MYNET_STAGING_FRESH=1
./scripts/fresh-staging.sh
export MYNET_STAGING_I_UNDERSTAND=1
./scripts/run-acceptance.sh
```

## Resume run (keep current staging DB and clients)

Do **not** run `fresh-staging` or `MYNET_STAGING_FRESH` if peers are already up.

```bash
cd deploy/staging
git pull
docker compose build
export MYNET_STAGING_I_UNDERSTAND=1
./scripts/run-acceptance.sh
```

Or stepwise:

```bash
docker compose up -d coordinator proxy
./scripts/bootstrap-coordinator.sh
./scripts/enroll-three.sh   # skips existing inv-*.txt
docker compose --profile clients up -d --build client-a client-b client-c
./scripts/data-plane.sh
./scripts/recovery.sh
./scripts/revoke.sh         # includes invitation-replay.sh
```

## Artifacts to review

| File | Pass criteria |
|------|----------------|
| `artifacts/last-error.txt` | Absent after success |
| `artifacts/data-plane-results.txt` | ICMP + `mynet ping` for A↔B, A↔C, B↔C |
| `artifacts/recovery-results.txt` | Coordinator health + ping after client-b restart |
| `artifacts/revoke-results.txt` | Revocation latencies; A↔B still up |
| `artifacts/invitation-replay-results.txt` | `PASS: coordinator rejected consumed invitation` |

Admin passwords and invitation tokens are **not** written to these logs.

## Pass / fail report template

```
Date (UTC):
Git revision:
Run type: [ ] fresh  [ ] resume

Bootstrap:           [ ] pass  [ ] fail
Three enrollments:   [ ] pass  [ ] fail
Data plane (3 pairs):[ ] pass  [ ] fail
Client-b restart:    [ ] pass  [ ] fail
Coordinator restart: [ ] pass  [ ] fail
Diagnose timings:    [ ] pass  [ ] n/a only where expected
Revocation:          [ ] pass  [ ] fail  (ping block: ___s, wg remove: ___s)
Invitation replay:   [ ] pass  [ ] fail  (coordinator http 410 / exhausted)
Notes:
```

## Local checks without privileged clients

```bash
go test ./... -count=1
go test -race ./... -count=1
./deploy/staging/scripts/bootstrap_selftest.sh
./deploy/staging/scripts/admin_session_selftest.sh
bash -n deploy/staging/scripts/*.sh
```
