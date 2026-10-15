# VPN CIDR migration: `10.77.0.0/16` → `172.31.240.0/20`

## Why

The lab Wi‑Fi uses a connected `10.0.0.0/8` route. Virtual addresses in `10.77.0.0/16` overlap that prefix and can be steered onto `en0` instead of the MyNet TUN.

Target pool: **`172.31.240.0/20`** (hosts `172.31.240.1`–`172.31.255.254`).

## What is preserved

- Device IDs, Ed25519 enrollment keys, WireGuard key pairs
- VPN registration (`wg_public_key` rows stay authorized)
- Stable addressing where explicitly assigned (Mac `.1`, mini PC `.2`)

## Preconditions (all devices)

1. **Stop VPN** on every node: `sudo -E mynet down` (daemon may keep running).
2. Optionally stop daemon: `mynet daemon stop`.
3. **Back up** coordination SQLite on the server:
   ```bash
   cp ~/mynet/devices.db ~/mynet/devices.db.bak.$(date +%Y%m%d%H%M)
   ```
4. Back up local state on each client:
   ```bash
   tar czf ~/mynet-state.bak.tgz ~/.mynet
   ```

## Server configuration change

Set the pool **before** or **with** the DB update (restart required):

```bash
export MYNET_VPN_CIDR=172.31.240.0/20
# restart mynet-server (see deploy/minipc-server-start.md)
```

Default in `mynet-server` binaries built for V0.2.1 is already `172.31.240.0/20` when the env var is unset.

## Database migration (approval required)

Inspect current rows:

```sql
sqlite3 ~/mynet/devices.db "SELECT device_id, name, vpn_ipv4, wg_public_key FROM devices WHERE revoked=0;"
```

Assign explicit addresses (replace `device_id` values from the query above):

```sql
BEGIN;
UPDATE devices SET vpn_ipv4='172.31.240.1' WHERE device_id='<macbook-device-id>';
UPDATE devices SET vpn_ipv4='172.31.240.2' WHERE device_id='<minipc-device-id>';
COMMIT;
```

Verify:

```sql
SELECT name, vpn_ipv4 FROM devices WHERE revoked=0;
```

### Rollback

```bash
cp ~/mynet/devices.db.bak.<timestamp> ~/mynet/devices.db
export MYNET_VPN_CIDR=10.77.0.0/16
# restart mynet-server
```

Clients keep old `wg.json` `vpn_ipv4` until `mynet vpn register` or `mynet up` (daemon syncs from `/v1/vpn/config`).

## Client refresh

On each device after server migration:

```bash
./bin/mynet vpn register   # refreshes wg.json vpn_ipv4 from server
./bin/mynet preflight        # should show 172.31.240.x peers, no 10.x overlap
```

`mynet up` also writes the server-assigned address into `wg.json` if it was stale.

## Re-allocation behavior (no manual SQL)

If a device still has `10.77.x` in SQLite when the server pool is `172.31.240.0/20`, the next `vpn/register` or allocation call **clears** the out-of-pool address and picks the next free host in the new pool. Use explicit SQL above to keep `.1` / `.2`.

## Live test sequence (after approval)

1. Server: backup DB, apply SQL, set `MYNET_VPN_CIDR`, restart server.
2. Mac + mini PC: deploy V0.2.1 binaries, `mynet vpn register`, `mynet preflight`.
3. Mac: `sudo -E mynet up` → `mynet status` (VPN up, peer may be Waiting).
4. Mini PC: `sudo -E mynet up` → peers should reach **Transport ready**.
5. `ping 172.31.240.x`, `ssh` over virtual IPs.
