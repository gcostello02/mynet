# MyNet

MyNet is a **fully self-hosted** mesh VPN. You run your own **coordinator** on a Linux machine you control; there is no MyNet-operated cloud, account, or control plane. Edge devices (macOS or Linux) join your coordinator over HTTPS, then connect peer-to-peer using WireGuard over ICE.

**Typical home layout**

| Machine | Runs |
|---------|------|
| Linux mini PC | Coordinator (Docker) + embedded web dashboard + Linux edge daemon |
| MacBook | macOS edge daemon only |
| Tailscale | Optional — use separately for admin SSH; MyNet does not modify it |

Module: `github.com/gcostello02/mynet`

## What runs where

**Coordinator (one per network)**

- Signaling, enrollment, admin API, SQLite state, coordinator Ed25519 identity
- Embedded admin UI at `/` (optional; setup works via CLI)
- Deploy with Docker: [deploy/coordinator](deploy/coordinator/README.md)

**Edge device**

- `mynet` CLI + background daemon (`mynet-daemon`)
- Device identity in `~/.mynet/`
- WireGuard TUN and per-peer routes when VPN is up (`mynet up`)

## 1. Install the coordinator (Linux)

```bash
git clone https://github.com/gcostello02/mynet.git
cd mynet/deploy/coordinator
cp .env.example .env
# MYNET_SERVER_SECRET=$(openssl rand -base64 32)  → paste into .env
docker compose build && docker compose up -d
curl -sf http://127.0.0.1:8080/healthz
```

Bind is **127.0.0.1:8080** by default. Terminate TLS with your own reverse proxy (nginx, Caddy, etc.) and your own hostname — no MyNet domain required.

Full reference: [deploy/coordinator/README.md](deploy/coordinator/README.md)

## 2. HTTPS and public URL

Configure your proxy to forward `https://your-host/` → `http://127.0.0.1:8080`. Set `MYNET_SECURE_COOKIES=true` in `.env` when using HTTPS.

Clients and invitations must use the **same HTTPS URL** you declare during setup (`--public-url`).

## 3. Initialize the administrator (CLI)

On the coordinator host:

```bash
# Build or install mynet on the host, or copy bin/mynet from the repo
mynet admin setup \
  --server http://127.0.0.1:8080 \
  --user admin \
  --network "My Network" \
  --public-url https://your-host.example
```

Password: minimum 12 characters. The browser setup wizard at `/` is optional.

Remote setup: `mynet admin bootstrap --server http://127.0.0.1:8080` on the host, then `mynet admin setup --bootstrap-token '…' --server https://…`.

## 4. Open the dashboard

Visit `https://your-host.example/` and sign in with the admin account. API routes remain under `/v1/*`.

## 5. Create an invitation

```bash
mynet admin login --server https://your-host.example --user admin
mynet admin password change   # prompts securely; invalidates all admin sessions
mynet admin invitations create --label laptop --max-uses 1
```

Copy the one-time invitation string; treat it like a password.

## 6. Join an edge device

**Linux or macOS** (after checking existing state: `ls ~/.mynet`, `mynet service status`):

```bash
mynet join '<invitation>' --name my-device
sudo mynet service install
sudo mynet service start
mynet service status
sudo mynet up
```

Details: [docs/v040-m2-join.md](docs/v040-m2-join.md)

## 7. Persistent edge service

| Command | Purpose |
|---------|---------|
| `sudo mynet service install` | Write systemd (Linux) or launchd (macOS) unit |
| `sudo mynet service start` | Start supervised daemon |
| `sudo mynet service stop` | Stop daemon |
| `mynet service status` | Health and VPN summary |
| `sudo mynet service upgrade --candidate ./bin/mynet` | Replace supervised binary |

Do not run `service install` blindly if another MyNet install already owns `mynet-daemon` — inspect first.

## Backup coordinator state

```bash
cd deploy/coordinator
docker compose exec coordinator tar -czf - -C /data . > backup.tar.gz
```

Back up `coordinator.db` and `coordinator.ed25519` together.

## Mini PC migration (V0.3 → V0.4)

If you previously ran native `mynet-server` on `:8787` with `devices.db`, read [deploy/coordinator/minipc.md](deploy/coordinator/minipc.md) for inspection commands, backups, port/tunnel changes, and re-enrollment.

## Troubleshooting

| Issue | What to check |
|-------|----------------|
| `/healthz` fails | `docker compose logs coordinator`, `MYNET_SERVER_SECRET` in `.env` |
| Setup forbidden | Use loopback on host, or bootstrap token for remote HTTPS |
| Join fails | HTTPS URL, invitation expiry, clock skew |
| VPN no connectivity | `mynet diagnose`, STUN/TURN in coordinator `.env`, firewall UDP |
| Admin locked out | Restore DB backup; no cloud recovery |

More: [docs/m1-coordinator-admin.md](docs/m1-coordinator-admin.md), [docs/v040-m3-first-run-setup.md](docs/v040-m3-first-run-setup.md)

## Build from source

```bash
make build-dashboard   # embed admin UI
make build             # bin/mynet, bin/mynet-server
```

Go 1.22+ (toolchain may auto-download). VPN: IPv4, per-peer `/32` routes — see [docs/vpn-setup.md](docs/vpn-setup.md).

## Developer / staging

Multi-container acceptance and CI images: [deploy/staging](deploy/staging/) and [docs/m040-m2-staging.md](docs/m040-m2-staging.md). Not required for production coordinator install.

`deploy/self-hosted/` is a compatibility alias for `deploy/coordinator/`.

## Documentation index

- [docs/v040-architecture.md](docs/v040-architecture.md) — V0.4 design
- [docs/v040-security-model.md](docs/v040-security-model.md)
- [docs/v040-m3-dashboard.md](docs/v040-m3-dashboard.md) — admin UI
- [docs/testing.md](docs/testing.md)

## License

TBD — add a license file before public release.
