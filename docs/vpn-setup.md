# VPN setup (V0.2)

## Prerequisites

- V0.1.2 coordination working (`https://mesh.example.com` or your host).
- `mynet config independent --server https://…` on each peer.
- **Root/admin** for `mynet up` (TUN + routes).
- Both peers enrolled and able to `mynet login`.

## Commands

```bash
# Each device (once)
mynet login

# Start VPN (starts background daemon if needed)
sudo mynet up

# Status
mynet status

# Stop VPN
sudo mynet down
```

## Addressing

The coordination server allocates stable IPv4 addresses from `10.77.0.0/16` (configurable via `MYNET_VPN_CIDR` on the server). WireGuard public keys are registered with `POST /v1/vpn/register` (signed by the device Ed25519 key).

## Both peers must be up

For a two-node mesh, run `sudo mynet up` on **both** MacBook and mini PC. ICE glare is avoided by having the lexicographically lower `device_id` initiate the session.

## Rollback

```bash
sudo mynet down
```

This removes per-peer `/32` routes installed by MyNet and closes the TUN interface. Tailscale and default routes are not modified.
