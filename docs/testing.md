# Testing

## Automated

```bash
make test            # go test ./... -race -count=1
```

The suite needs no network, root or real NAT. It covers:

- Identity persistence and signing, invitation parsing and verification
- Coordinator enrollment, auth, admin API, setup and bootstrap, rate limits, revocation, and SQLite migrations
- VPN address allocation, WireGuard engine construction, and the `ICEBind` datagram relay
- Local ICE pairs and the full stack of HTTP server, WebSocket clients, ICE and WireGuard
- Daemon lifecycle, supervised service layout, and upgrade safety checks
- Dashboard handler (security headers, SPA routing)

STUN isn't contacted in unit tests. A few timing-sensitive tests in `internal/wgbind` can flake when the whole suite runs in parallel under `-race` on a busy machine; rerun the package on its own (`go test ./internal/wgbind -race -count=1`) before treating a failure as real.

Dashboard tests:

```bash
cd web/admin && npm ci && npm run typecheck && npm test
```

CI (`.github/workflows/ci.yml`) runs on pushes to `main` and on pull requests. It typechecks, tests and builds the dashboard; checks `gofmt` and `go vet`; runs the Go tests with the race detector; and cross-compiles `mynet` and `mynet-server` for linux/amd64 and linux/arm64 (running the Linux-only `netprobe` tests natively on amd64). Run `make fmt` before pushing if the formatting check fails.

## Manual checks on real devices

These need root for the TUN device and routes, so they aren't part of CI.

1. Enroll two or more devices ([joining-devices.md](joining-devices.md)) and start the VPN on each (`sudo mynet service start`).
2. `mynet status` on each: `WireGuard: Active`, every peer `Connected` with `Transport ready`.
3. `ping <peer MyNet IP>` in both directions, then `ssh user@<peer MyNet IP>`.
4. **Cross-network:** put one device on a phone hotspot and repeat. Expect a `Direct P2P` path via STUN, or `Relayed (TURN)` if you've set up a relay.
5. **Relay:** `mynet connect --peer <name> --relay-only` should report `TURN Used: true`, or fail clearly when TURN is unreachable ([networking.md](networking.md#testing-the-relay)).
6. **Recovery:** reboot a device (the VPN should return on its own), restart the coordinator (devices should reconnect), and revoke a device (peers drop it within about 45 s).

## Staging harness

For end-to-end validation of real ICE, WireGuard, recovery, revocation and invitation replay in isolated containers, see [staging.md](staging.md).

## Profiling

See [profiling.md](profiling.md). It's opt-in and meant for development only.
