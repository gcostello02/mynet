# Self-hosted coordinator (Docker Compose)

Run your own MyNet coordinator with the embedded admin UI. This stack uses **`deploy/self-hosted/Dockerfile`** (coordinator entrypoint only). It does **not** use the staging VPN client image entrypoint.

## Prerequisites

- Docker Engine with Compose v2
- HTTPS reverse proxy for the public coordinator URL (production clients require HTTPS)
- Optional: STUN/TURN for ICE

## Quick start

```bash
cd deploy/self-hosted
cp .env.example .env
# Edit .env — set MYNET_SERVER_SECRET (openssl rand -base64 32)
docker compose build
docker compose up -d
docker compose logs coordinator   # copy one-time bootstrap capability from stderr
```

Open **on the coordinator host**: `http://127.0.0.1:8080/` (default bind is localhost only).

Complete the setup wizard. For remote setup, paste the bootstrap capability from the logs into the wizard.

Optional CLI (loopback on the host):

```bash
mynet admin bootstrap --server http://127.0.0.1:8080
```

## Environment

| Variable | Required | Notes |
|----------|----------|--------|
| `MYNET_SERVER_SECRET` | Yes | ≥ 32 bytes; `openssl rand -base64 32` |
| `MYNET_DATA_PATH` | No | Default `/data/coordinator.db` in container |
| `MYNET_SECURE_COOKIES` | No | `true` behind HTTPS |
| `MYNET_HOST_PORT` | No | Host port (default `8080`), bound to `127.0.0.1` |
| `MYNET_VPN_CIDR` | No | VPN pool for enrolled devices |
| `MYNET_STUN_URLS` / `MYNET_TURN_*` | No | ICE helpers |

**Not required:** `MYNET_CLIENT_NAME`, invitation artifacts, or any staging client variables.

## Persistent state

Volume `coordinator-data` → `/data`:

- `coordinator.db` — SQLite
- `coordinator.ed25519` — coordinator private key (`0600`)

Back up both before upgrades.

## Health & restarts

- `GET /healthz` — liveness (used by Compose healthcheck)
- `restart: unless-stopped` — container restarts on failure; fix config if it loops (e.g. missing `MYNET_SERVER_SECRET`)

## Smoke test

Isolated build/start check (own project name + volume; does not touch staging):

```bash
./scripts/smoke-test.sh
```

## Upgrades

1. Back up the volume.
2. `docker compose build && docker compose up -d`

See [docs/v040-m3-first-run-setup.md](../../docs/v040-m3-first-run-setup.md) for bootstrap security and staging validation boundaries.
