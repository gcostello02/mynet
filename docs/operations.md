# Operations

Day-to-day running, diagnostics and recovery for edge devices. For setting up the coordinator, see [deploy/coordinator/README.md](../deploy/coordinator/README.md).

## Service and VPN commands

| Command | Effect |
|---------|--------|
| `sudo mynet service install` | Write the systemd (Linux) or launchd (macOS) unit |
| `sudo mynet service start` | Start the supervised daemon (Linux: also enable it at boot) |
| `sudo mynet service stop` / `restart` | Stop or restart it |
| `mynet service status` | Daemon, coordinator link and VPN summary |
| `mynet service logs` | Recent daemon logs |
| `sudo mynet service upgrade --candidate ./bin/mynet` | Replace the supervised binary |
| `sudo mynet service uninstall` | Remove the unit |
| `sudo mynet up` | Bring the VPN up (starts the daemon if it isn't running) |
| `sudo mynet down` | Stop the VPN only (TUN, routes, WireGuard, peer workers). Signaling and the daemon keep running |
| `mynet daemon stop` | Full shutdown: VPN, signaling and the daemon process |

With the service installed, the daemon brings the VPN up on its own at start, so `mynet up` is only needed for manual use.

## Checking health

```bash
mynet status                     # device, virtual IP, peers, transport (Direct P2P / Relayed)
mynet peers                      # peers and whether they're online
mynet diagnose --peer <name>     # detailed ICE and timing for one peer
mynet preflight                  # checks before running the VPN
```

Healthy `mynet status` shows `Coordination: Connected`, `WireGuard: Active`, and each peer `Connected` with `Transport ready`.

### Reading `mynet diagnose`

- **Connection timing** covers the last ICE attempt: signaling ready → session created → remote credentials → gather → ICE connect → data transport ready. Durations are elapsed time on this device, not clock differences between machines.
- **Current error** applies only while the peer path isn't ready. **Last failure** is historical. **Last success** is when the link last completed ICE.
- **Transport state** is the live ICE state of the nominated connection.

## Addressing

The coordinator allocates each device a stable IPv4 address from `MYNET_VPN_CIDR` (default `172.31.240.0/20`). Each peer is reached via a `/32` route to the TUN interface (`mynet0` on Linux, `utunN` on macOS). MyNet doesn't change the default route or DNS. Pick a CIDR that doesn't overlap networks your devices sit on, such as a LAN, VPN or Docker bridges.

## Troubleshooting

| Symptom | Check |
|---------|-------|
| `tun (requires root)` | Run with `sudo mynet up` |
| Peer stays `Connecting` | The other device must be online with its daemon running. Check `mynet diagnose --peer <name>`, UDP isn't blocked, and STUN/TURN are reachable ([networking.md](networking.md)) |
| `Daemon IPC: false` right after `service start` | The daemon is still starting; re-run `mynet status` in a few seconds |
| `vpn not registered` | Re-join (`mynet join`), or run `mynet vpn register` |
| Can't reach a peer's MyNet IP, but status says Connected | Check the route (`ip route \| grep 172.31.240` or `netstat -rn \| grep 172.31.240`) and that the peer's firewall allows the service |
| Handshake works but large TCP transfers stall | MTU: the interface uses 1420; test `ping -s 1200` |
| Device dropped off after reboot (Linux) | `systemctl is-enabled mynet-daemon` should say `enabled`; fix with `sudo systemctl enable mynet-daemon` |
| Authentication errors after a long uptime | Device tokens renew automatically; if the device was revoked, it must be re-invited |
| `mynet status` fails with a permission error | A root-owned `mynetd.sock`; the daemon normally chowns it to the service owner. Restart the service |

## Recovery

### The daemon died and left a TUN or routes behind

1. Confirm it's gone: `mynet status` shouldn't show the VPN up.
2. Remove stale IPC files (only if no daemon is running): `rm -f ~/.mynet/mynetd.sock ~/.mynet/mynetd.pid`
3. Remove leftover routes:
   - **Linux:** `ip route | grep 172.31.240`, then `sudo ip route del <ip>/32 dev mynet0`
   - **macOS:** `netstat -rn | grep 172.31.240`, then `sudo route -n delete -host <peer-ip>`; if a `utun` still has your MyNet address, `sudo ifconfig <utun> down`
4. Restart: `sudo mynet service restart` (or `sudo mynet up`).

### `mynet down` printed `killed` on macOS

`down` only sends a request to the daemon, which does the teardown. If the CLI process is killed before it prints the reply, you can see `killed` even though teardown continues. Run `mynet status`; if the VPN is still up, retry `sudo mynet down`, or `mynet daemon stop` to stop everything.

### Running with sudo

Run enrollment (`mynet join`) as your normal user so keys land in `~/.mynet`. For manual `sudo mynet up`, use `sudo -E` (or set `MYNET_STATE_DIR=$HOME/.mynet`) so root finds your identity. The installed service handles this for you via `MYNET_STATE_DIR` and `MYNET_OWNER_USER`.

## Environment variables (edge)

| Variable | Purpose |
|----------|---------|
| `MYNET_STATE_DIR` | State directory (default `~/.mynet`); needed for root services |
| `MYNET_SUPERVISED` | Set by systemd/launchd |
| `MYNET_DEBUG` | Verbose logging |
| `MYNET_ENABLE_PPROF` | Opt-in profiling, see [profiling.md](profiling.md) |

## State files

| File | Purpose |
|------|---------|
| `identity.json` | Ed25519 device identity (0600) |
| `wg.json` | WireGuard key (0600) |
| `session.json` | Device bearer token |
| `config.json` | Server URL and network options |
| `admin-session.json` | Admin CLI session (only if you ran `mynet admin login`) |
| `mynetd.sock`, `mynetd.pid` | Daemon IPC |

## Backing up and upgrading the coordinator

See [deploy/coordinator/README.md](../deploy/coordinator/README.md). The database and `coordinator.ed25519` must be backed up together.
