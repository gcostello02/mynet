# VPN troubleshooting

| Symptom | Check |
|---------|--------|
| `tun (requires root)` | Run `sudo mynet up` |
| Peer `timed out waiting for ICE` | Other peer must run `sudo mynet up`; only one signaling session per device |
| `vpn not registered` | `mynet login` then `mynet up` (registers WG key) |
| Ping works on Tailscale IP but not MyNet IP | Confirm `mynet status` shows peer Connected; check `/32` route via `netstat -rn` |
| Handshake but no TCP | MTU — interface uses 1420; try `ping -s 1200` |
| TURN-only networks | External TURN must be on a public IP, not `100.x` |

## Crash recovery (daemon died, TUN/routes left behind)

1. Confirm daemon is gone: `mynet status` (should not show VPN up).
2. Remove stale IPC (safe if no `mynetd` process): `rm -f ~/.mynet/mynetd.sock ~/.mynet/mynetd.pid`
3. **macOS** — list MyNet routes: `netstat -rn | grep 10.77`
   - Delete a stale host route: `sudo route -n delete -host <peer-mynet-ip>`
   - Remove utun if still up: `ifconfig` → find `utun*` with `10.77.x.x`, then `sudo ifconfig <utun> down` (only if it is the MyNet address from `mynet preflight` backup notes).
4. **Linux** — `ip route | grep 10.77`; delete with `sudo ip route del <ip>/32 dev <ifname>`.
5. MyNet does **not** change default route or DNS; Tailscale is untouched.
6. Restart: `mynet preflight`, then `sudo -E mynet up`.

## Privileged path mistakes

| Mistake | Effect |
|---------|--------|
| `sudo mynet init` / `login` without `-E` | Writes under `/var/root` — use normal user for enrollment |
| `sudo mynet up` without `SUDO_USER` | Identity missing error — use `sudo -E` or set `MYNET_STATE_DIR=$HOME/.mynet` |
| Root-owned `mynetd.sock` | `mynet status` fails — daemon `chown`s socket to `SUDO_USER` when started via `sudo -E mynet up` |

Recovery: `sudo -E mynet down`, verify Tailscale (`tailscale status`) still works, then `sudo -E mynet up`.
