# Coordinator deployment (Docker)

Run the MyNet **coordinator** on a Linux server you control. One container runs:

- `mynet-server` (signaling, enrollment API, admin API)
- Embedded web dashboard at `/` (optional; not required for setup)

Edge devices run the `mynet` client on the host (macOS or Linux), not in this stack.

## Quick start

```bash
git clone https://github.com/gcostello02/mynet.git
cd mynet/deploy/coordinator
cp .env.example .env
# Edit .env: set MYNET_SERVER_SECRET (openssl rand -base64 32)
docker compose build
docker compose up -d
curl -sf http://127.0.0.1:8080/healthz   # ok
```

Bind defaults to **127.0.0.1:8080** on the host. Put **nginx**, **Caddy**, or another reverse proxy in front for HTTPS. Set `MYNET_SECURE_COOKIES=true` when the dashboard is served over HTTPS.

## First-run admin (CLI)

On the **coordinator host** (loopback is trusted):

```bash
mynet admin setup \
  --server http://127.0.0.1:8080 \
  --user admin \
  --network "Home" \
  --public-url https://mesh.example.com
```

Password: at least 12 characters. This creates the coordinator Ed25519 identity, SQLite state, and the first admin account.

Remote setup (from another machine) requires a one-time bootstrap token:

```bash
mynet admin bootstrap --server http://127.0.0.1:8080   # run on host via SSH
mynet admin setup --server https://mesh.example.com --bootstrap-token '<token>' ...
```

The browser setup wizard at `/` is optional.

## Invitations and dashboard

```bash
mynet admin login --server https://mesh.example.com --user admin
mynet admin invitations create --label laptop --max-uses 1
```

Open `https://mesh.example.com/` in a browser to use the dashboard (after login).

## Persistent state

Docker volume `coordinator-data` maps to `/data` in the container:

| File | Purpose |
|------|---------|
| `coordinator.db` | SQLite (devices, admins, invitations, meta) |
| `coordinator.ed25519` | Coordinator signing key (backup with the DB) |

Backup (coordinator **stopped** or consistent snapshot):

```bash
docker compose exec coordinator tar -czf - -C /data . > mynet-coordinator-backup-$(date +%Y%m%d).tar.gz
```

Restore into a new volume before first start, or replace files under the volume mount.

## Environment

See `.env.example`. Important variables:

| Variable | Notes |
|----------|--------|
| `MYNET_SERVER_SECRET` | Required, ≥32 bytes |
| `MYNET_HOST_PORT` | Host port (default 8080), localhost only |
| `MYNET_VPN_CIDR` | Default `172.31.240.0/20` |
| `MYNET_STUN_URLS` / `MYNET_TURN_*` | Optional NAT traversal |

## Upgrade

```bash
cd deploy/coordinator
git pull
docker compose build
docker compose up -d
```

Identity and DB persist across rebuilds.

## Smoke test

```bash
./scripts/smoke-test.sh
```

## Developer staging

CI and multi-container acceptance tests use [deploy/staging](../staging/) — not required for production install.
