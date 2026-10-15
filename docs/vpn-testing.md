# VPN testing

## Automated (CI)

- Virtual IP allocation (`internal/control/server/allocate_test.go`)
- ICE bind datagram relay (`internal/wgbind/icebind_test.go`)
- WireGuard device construction (`internal/wgengine/engine_test.go`)
- SQLite migration for `wg_public_key` / `vpn_ipv4`

## Manual (requires root)

1. Deploy updated `mynet-server` with SQLite path.
2. On mini PC and Mac: `mynet login`, then `sudo mynet up` on **both**.
3. `mynet status` — expect `WireGuard: Active`, peers `Connected`.
4. `ping 10.77.0.2` (from Mac) / `ping 10.77.0.1` (from mini PC).
5. `ssh user@10.77.0.2` over MyNet IP.

## Cross-network

Repeat from phone hotspot with independent mode. Confirm ICE path in logs (`MYNET_DEBUG=1`).

## Not verified in CI

- Live TUN on macOS/Linux without sudo
- End-to-end SSH across real WAN (requires your approval for host networking)
