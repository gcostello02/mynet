# MyNet V0.4 — Product vision

## What MyNet is

MyNet is **100% self-hosted mesh VPN software** (no mandatory MyNet cloud, accounts, or hosted coordinator): WireGuard encryption over ICE/STUN-discovered UDP paths, with a **coordination server** you operate for enrollment, peer directory, and signaling. Peer data does not transit the coordinator except when TURN relay is required.

V0.3.0 proved always-on clients (launchd/systemd), multi-peer WireGuard, network-change recovery, and independent connectivity without Tailscale.

## Who it is for

Technical or moderately technical users who want:

- A private network they control (no SaaS control plane)
- macOS and Linux clients first; Windows later
- Automatic reconnect across reboots and network changes
- Direct P2P when possible; optional TURN when not

## Target experience (V0.4 direction)

1. **Install** MyNet client and (optionally) coordinator on chosen host.
2. **Create network** or **join** with an invitation.
3. **Pick coordinator host** and complete reachability setup (HTTPS/WSS).
4. **Bootstrap admin** (first operator identity) with protected one-time setup.
5. **Invite devices** via short-lived enrollment tokens.
6. **Enroll** additional machines; receive VPN address and peer list automatically.
7. **Manage** devices (list, rename, revoke) from a **web dashboard** and CLI.
8. **Stay connected** via supervised daemons and existing ICE/VPN recovery.

## Non-goals for V0.4

- Hosted multi-tenant SaaS or MyNet-operated coordinators
- Consumer “one click” installers for all platforms (stretch: guided scripts)
- Full zero-trust policy engine, SSO, or enterprise MDM
- Replacing Tailscale for admin access (keep independent SSH/recovery paths)

## Success criteria

- A new operator can stand up a coordinator and three clients without editing Go source or hand-editing JSON secrets.
- Admin can revoke a device; revoked clients cannot authenticate.
- Dashboard shows **signaling presence** separately from **VPN data-path health**.
- macOS/Linux supervised install paths remain root-owned libexec/systemd patterns from V0.3.0.
