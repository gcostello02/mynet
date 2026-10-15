# VPN preflight and first live test

## Privileges

| Action | Privilege |
|--------|-----------|
| `mynet preflight`, `mynet login`, daemon IPC | User |
| `mynet up` (TUN + routes) | **root** (`sudo`) on macOS/Linux |

Under `sudo`, MyNet resolves config from **`SUDO_USER`**’s `~/.mynet` (not `/var/root`). Override with:

```bash
export MYNET_STATE_DIR="$HOME/.mynet"
sudo -E mynet up
```

## Coordination server must be V0.2+

`mynet preflight` calls `GET /v1/vpn/config`. From any machine:

```bash
curl -s -o /dev/null -w "%{http_code}\n" https://mesh.example.com/v1/vpn/config
```

| Code | Meaning |
|------|---------|
| **401** | VPN routes exist (missing bearer token) — server is new enough |
| **404** | Old `mynet-server` — rebuild and restart on the origin (mini PC), same `MYNET_DATA_PATH` and secrets |

After upgrading the binary on the mini PC (keep SQLite + `MYNET_SERVER_SECRET` + `MYNET_ENROLLMENT_SECRET`):

```bash
# on mini PC (example)
cd ~/mynet && make build
# stop existing mynet-server, then:
./bin/mynet-server --listen :8787 --data /path/to/devices.db
```

Re-check: `curl -s -o /dev/null -w "%{http_code}\n" https://mesh.example.com/v1/vpn/config` → **401**.

## Mini PC procedure

```bash
# Backup
cp -a ~/.mynet ~/.mynet.bak.$(date +%Y%m%d)
cp -a /path/to/mynet-server.db /path/to/mynet-server.db.bak.$(date +%Y%m%d)

cd /path/to/mynet && make build
# Restart mynet-server (preserve env + SQLite); do not touch cloudflared

curl -fsS https://mesh.example.com/healthz   # expect: ok
curl -s -o /dev/null -w "vpn_api=%{http_code}\n" https://mesh.example.com/v1/vpn/config   # expect: 401

mynet config independent --server https://mesh.example.com
mynet login
mynet preflight    # paths + planned /32 routes (no changes)

# After approval:
sudo -E mynet up
mynet status
```

## MacBook procedure

```bash
cp -a ~/.mynet ~/.mynet.bak.$(date +%Y%m%d)
cd /path/to/mynet && make build

mynet config show   # independent + mesh.example.com
mynet preflight
sudo -E mynet up
mynet status
```

## First acceptance tests

Use addresses from `mynet status` (server-allocated, not assumed):

```bash
ping -c 3 <minipc-vpn-ip>
ssh youruser@<minipc-vpn-ip>
```

## Diagnostics

```bash
mynet diagnose --peer minipc
MYNET_DEBUG=1 mynet status
# macOS: netstat -rn | grep 10.77
# Linux: ip route | grep 10.77
```

## Rollback

```bash
sudo -E mynet down
# If routes remain (crash): see docs/vpn-troubleshooting.md
# Tailscale: tailscale status (unchanged)
```
