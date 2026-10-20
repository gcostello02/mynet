# Self-hosted coordinator (Docker Compose)

This path is for operators who download MyNet and run their own coordinator. No MyNet cloud account is required.

## Prerequisites

- Docker Engine with Compose v2
- A reverse proxy terminating **HTTPS** for the public coordinator URL (Caddy, nginx, Traefik, etc.)
- UDP reachability for WireGuard and (optionally) STUN/TURN for ICE

## Quick start

1. Copy `.env.example` to `.env` and set `MYNET_SERVER_SECRET` (`openssl rand -base64 32`).
2. `docker compose up -d`
3. Read the **bootstrap capability** printed in `docker compose logs coordinator` (one-time, expires in 2 hours).
4. Open `http://127.0.0.1:8080/` on the host (or tunnel) and complete the setup wizard.
5. Point your reverse proxy at `127.0.0.1:8080` only after you understand exposure risk; use HTTPS and restrict admin paths if needed.

## Persistent state

The `coordinator-data` volume contains:

| Path | Purpose |
|------|---------|
| `coordinator.db` | SQLite (devices, invitations, admins, network meta) |
| `coordinator.ed25519` | Coordinator private key (mode `0600`) |

**Back up both** before upgrades or migration. Restoring requires matching files from the same installation.

## Bootstrap capability

- Issued when the database is uninitialized; rotated on each coordinator start until setup completes.
- Shown on **stderr** at startup and via loopback `GET /v1/setup/bootstrap` or `mynet admin bootstrap --server http://127.0.0.1:8080`.
- Required in header `X-Mynet-Bootstrap-Token` for remote `POST /v1/admin/setup` (non-loopback).
- Never logged through structured logging; do not put it in URLs.

## Upgrades

1. Back up the volume.
2. Pull/build the new image.
3. `docker compose up -d` (same volume). Setup cannot run again once initialized.

## Health

- `GET /healthz` — liveness
- `GET /v1/setup/status` — `{ "initialized": true|false, ... }` (no secrets)

See [docs/v040-m3-first-run-setup.md](../../docs/v040-m3-first-run-setup.md) for threat model and staging validation (no production changes).
