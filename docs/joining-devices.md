# Joining devices

Adding a device to your network takes an invitation from an admin and a `mynet join` on the device.

## 1. Create an invitation

From any machine with the `mynet` CLI:

```bash
mynet admin login --server https://your-host.example --user admin
mynet admin invitations create --label laptop --max-uses 1
```

| Flag | Default | Meaning |
|------|---------|---------|
| `--label` | none | Name shown in the dashboard |
| `--max-uses` | `1` | How many devices can enroll with it |
| `--expires` | `86400` | Lifetime in seconds (24 h) |

Or use the dashboard: **Invitations → create → Copy join command**. Either way, the invitation is shown once and is a secret: deliver it out of band (a password manager, an encrypted message), not over email or a shared doc.

Other admin commands:

```bash
mynet admin invitations list
mynet admin devices list
mynet admin devices revoke <device_id>
mynet admin password change      # invalidates all admin sessions
mynet admin logout
```

Admin credentials are stored in `~/.mynet/admin-session.json`, separate from device credentials.

## 2. Join the device

Check for existing state first (`ls ~/.mynet`, `mynet service status`). Don't join a device that already belongs to another MyNet network, because `join` refuses to replace an identity unless you pass `--force`.

```bash
mynet join --name my-laptop '<invitation>'
```

Flags may come before or after the invitation. Without an invitation argument, `mynet join` prompts for one on stdin. The invitation can be the compact base64url string the dashboard and CLI produce, or its JSON form.

`join` does these things:

1. Parses the invitation and requires an **HTTPS** coordinator URL.
2. Checks `GET /v1/network/public` against the invitation's network ID and coordinator fingerprint.
3. Sends a random challenge to `POST /v1/network/proof` and verifies the coordinator's Ed25519 signature. Redirects to another host are rejected.
4. Generates device keys locally, enrolls, and registers its WireGuard key.

`join` doesn't start the VPN. It also refuses to run while a supervised daemon is already running on the device.

## 3. Start MyNet

Linux or macOS, with root:

```bash
sudo mynet service install
sudo mynet service start     # Linux: also enables start at boot
sudo mynet up
mynet status
```

On macOS, launchd loads the daemon at boot. A manual alternative that doesn't install a service is `sudo -E mynet up` (the VPN stops when the daemon does).

Expected output:

```text
Device: my-laptop
Virtual IP: 172.31.240.2
WireGuard: Active

PEER     MYNET IP      TRANSPORT    ICE          STATE
minipc   172.31.240.1  Direct P2P   Connected    Transport ready
```

## Troubleshooting a join

| Error | Cause |
|-------|-------|
| `--name is required` | Pass `--name <device>` |
| `invitation is not valid JSON or base64url` | The string was truncated or reformatted; copy it again, in single quotes |
| Coordinator fingerprint mismatch | The URL doesn't point at the coordinator that issued the invitation |
| Invitation expired, or used up | Create a new invitation |
| `supervised daemon is running; stop it before join` | `sudo mynet service stop`, then join again |
| TLS or redirect errors | The coordinator must be reachable at the exact HTTPS URL used at setup (`--public-url`) |

For connectivity problems after joining, see [operations.md](operations.md).

## Removing a device

```bash
mynet admin devices revoke <device_id>      # on the admin machine, or use the dashboard
sudo mynet service uninstall                # on the device (optional)
```

Peers drop the revoked device within about 45 s. A revoked device can't re-enroll.
