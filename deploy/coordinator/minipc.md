# Mini PC — coordinator + Linux edge

Target layout:

| Component | Where |
|-----------|--------|
| Coordinator | Docker (`deploy/coordinator`) |
| Linux edge daemon | Host (`mynet` + `mynet-daemon.service`) |
| Admin access | Tailscale (unchanged) + optional HTTPS tunnel to `:8080` |
| MyNet VPN | WireGuard interface managed by `mynet` (default CIDR `172.31.240.0/20`) |

Do **not** stop Tailscale, Cloudflare Tunnel, or host routes unless you intend to change them.

## Phase 1 — Read-only inspection (safe)

Run on the mini PC:

```bash
# Old V0.3 native server?
pgrep -af 'mynet-server' || true
ss -tlnp | grep -E ':8787|:8080' || true
ls -la ~/mynet/devices.db ~/mynet/env.sh 2>/dev/null || true

# Edge daemon
systemctl status mynet-daemon.service 2>/dev/null || true
ls -la ~/.mynet/identity.json ~/.mynet/session.json 2>/dev/null || true

# Docker coordinator (if already present)
docker ps -a --filter name=mynet-coordinator 2>/dev/null || true
docker volume ls | grep coordinator || true

# VPN / WireGuard (MyNet — verify names before changing anything)
ip -br link show type wireguard 2>/dev/null || true
sudo wg show 2>/dev/null || true
```

Record: ports in use, paths to `devices.db` vs Docker volume, device names in `~/.mynet/`.

## Phase 2 — Backup (before replacement)

```bash
# V0.3 SQLite
test -f ~/mynet/devices.db && cp -a ~/mynet/devices.db ~/mynet/devices.db.bak-$(date +%Y%m%d)

# Edge identity (host)
tar -czf ~/mynet-edge-state-$(date +%Y%m%d).tar.gz -C "$HOME" .mynet 2>/dev/null || true

# Docker coordinator volume (if running)
cd ~/mynet/deploy/coordinator  # or your clone path
docker compose exec coordinator tar -czf - -C /data . > ~/mynet-coordinator-docker-$(date +%Y%m%d).tar.gz
```

## V0.3 → V0.4 conflicts

| V0.3 (legacy) | V0.4 (this guide) |
|---------------|-------------------|
| Native `mynet-server` on **:8787** | Docker coordinator on **127.0.0.1:8080** |
| `~/mynet/devices.db` | Docker volume → `coordinator.db` + `coordinator.ed25519` |
| `MYNET_ENROLLMENT_SECRET` / `mynet login` | Invitation-only after admin setup |
| Cloudflare → `http://127.0.0.1:8787` | Repoint proxy to `http://127.0.0.1:8080` |

**You cannot** run `mynet admin setup` on a database that already has enrolled devices. For a clean V0.4 network, use a **new** coordinator data volume (or empty DB), re-enroll all edges with **new invitations**, and retire the old server.

Migrating the same physical devices: revoke or abandon old enrollment, remove or archive `~/.mynet` on each edge if you want fresh device IDs, then `mynet join` again.

## Phase 3 — Deploy coordinator (operator)

```bash
cd ~/mynet
git fetch && git checkout feature/v040-m3-dashboard && git pull   # or your release branch
cd deploy/coordinator
cp -n .env.example .env
# Set MYNET_SERVER_SECRET in .env
docker compose build
docker compose up -d
curl -sf http://127.0.0.1:8080/healthz
```

Point your HTTPS reverse proxy (or Cloudflare tunnel) at `http://127.0.0.1:8080`. Update tunnel config from **8787 → 8080** when switching.

## Phase 4 — Initialize admin (on mini PC)

```bash
# Install mynet CLI on host if needed: make build && sudo install -m755 bin/mynet /usr/local/bin/mynet
mynet admin setup \
  --server http://127.0.0.1:8080 \
  --user admin \
  --network "Home" \
  --public-url https://mesh.example.com
```

## Phase 5 — Stop old coordinator (only after new one works)

```bash
pkill -f 'mynet-server --listen' || true
# Confirm nothing listens on 8787 unless you still need it
ss -tlnp | grep 8787 || true
```

## Phase 6 — Enroll mini PC as edge (host)

```bash
mynet admin login --server https://mesh.example.com --user admin
mynet admin invitations create --label minipc-edge --max-uses 1
# Copy the printed invitation string

mynet join '<invitation>' --name minipc
sudo mynet service install
sudo mynet service start
mynet service status
sudo mynet up    # or set VPN at start in service env
```

## Phase 7 — macOS edge (MacBook)

On the Mac (do not overwrite `~/.mynet` without checking):

```bash
mynet service status
ls -la ~/.mynet/

# New join (invitation from admin)
mynet join '<invitation>' --name macbook
sudo mynet service install
sudo mynet service start
mynet service status
sudo mynet up
```

## Service commands (both platforms)

| Action | Linux | macOS |
|--------|-------|-------|
| Install unit | `sudo mynet service install` | `sudo mynet service install` |
| Start | `sudo mynet service start` | `sudo mynet service start` |
| Stop | `sudo mynet service stop` | `sudo mynet service stop` |
| Status | `mynet service status` | `mynet service status` |
| Upgrade binary | `sudo mynet service upgrade --candidate ./bin/mynet` | same |

## Troubleshooting

| Symptom | Check |
|---------|--------|
| `setup already completed` | Fresh volume or intentional reuse |
| `existing enrolled devices` | New DB required for admin setup |
| Dashboard CSRF / login | `MYNET_SECURE_COOKIES=true` behind HTTPS |
| Edge cannot enroll | HTTPS `public-url`, invitation not expired, fingerprint pin |
| VPN up, no peers | `mynet peers`, coordinator reachability, revocation |

Legacy V0.3 server notes: [deploy/minipc-server-start.md](../minipc-server-start.md).
