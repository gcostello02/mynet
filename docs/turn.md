# TURN relay (optional)

MyNet clients gather ICE candidates locally. When direct host or STUN paths fail (symmetric NAT, UDP-blocking networks), a **TURN** relay can carry UDP.

## Server-issued credentials

When `mynet-server` has:

- `MYNET_TURN_SECRET` — shared secret (same as coturn `static-auth-secret`)
- `MYNET_TURN_URLS` — comma-separated base URLs, e.g. `turn:100.116.42.54:3478?transport=udp`

authenticated clients receive ephemeral username/password via `GET /v1/ice-servers` on login. Credentials use the coturn time-limited HMAC format (`<expiry>:<device_id>`).

Optional: `MYNET_STUN_URLS` overrides the default public STUN list returned to clients.

## coturn on the mini PC

```bash
cd deploy/coturn
# Edit turnserver.conf: set static-auth-secret to a long random string
export MYNET_TURN_SECRET='same-secret-here'
docker compose up -d
```

Add to `~/mynet/env.sh` on the server host:

```bash
export MYNET_TURN_SECRET='same-secret-here'
export MYNET_TURN_URLS='turn:100.116.42.54:3478?transport=udp'
```

Restart `mynet-server`. Clients only need to `mynet login` again to pick up TURN URLs (merged with local `stun_urls`).

## Verifying relay path

After connect, `mynet status` should show `relay (TURN)` for local or remote candidate type when the nominated pair uses TURN.

Tailscale development does **not** require TURN; see [tailscale.md](tailscale.md).
