# VPN security model

## Trust

- **Coordination server** is trusted for enrollment, virtual IP allocation, and distributing peer WireGuard public keys. A compromised server can assign wrong IPs or publish attacker keys (mitigated by TLS to Cloudflare origin and signed registration).
- **WireGuard** provides confidentiality and authenticity for traffic on the virtual network once peers have correct keys and `allowed_ips`.
- **ICE** provides NAT traversal only; it does not encrypt application IP packets.

## Keys

| Asset | Storage | Purpose |
|-------|---------|---------|
| Ed25519 device key | `~/.mynet/identity.json` (0600) | API auth, VPN register signatures |
| WireGuard private key | `~/.mynet/wg.json` (0600) | Tunnel encryption |

WireGuard private keys are **never** sent to the server.

## Registration binding

`POST /v1/vpn/register` requires an Ed25519 signature over `mynet-vpn-register-v1` including `device_id`, `wg_public_key`, and timestamp.

## Coordination compromise

If the server is compromised, an attacker could:

- Publish rogue WireGuard keys for peers (clients should pin known device names / monitor key changes — future hardening).
- Deny service or mis-allocate IPs.

They cannot decrypt past WireGuard traffic without peer private keys.
