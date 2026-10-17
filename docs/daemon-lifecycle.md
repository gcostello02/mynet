# Daemon lifecycle (mynetd)

## Commands

| Command | Effect |
|---------|--------|
| `mynet up` | Start `mynetd` if needed (detached session on Unix), then bring up VPN. Idempotent if VPN already running. |
| `mynet down` | Stop VPN (TUN, routes, WireGuard, peer workers). **Keeps** signaling and the IPC daemon running. |
| `mynet daemon stop` | Graceful full shutdown: VPN + signaling, close pprof socket, close IPC, exit process. |

## macOS `zsh: killed` (exit 137) on `mynet down`

**Observed:** The CLI sometimes exits with signal 9 before printing a result, while the root daemon keeps running and the VPN may stay up.

**Root cause (verified behavior, not a single kernel bug):**

1. `mynet down` only sends an IPC `down` request; cleanup runs **inside the daemon**. If the **CLI process** is killed (SIGKILL) before it prints the response, the user sees `killed` even though the daemon may still be running or still tearing down.
2. Previously, `mynet down` also **stopped signaling** (`sigCancel` + `rt.Down()`), which was incorrect for “VPN off, daemon on” semantics and could lengthen or complicate teardown.
3. `mynet daemon stop` only closed the IPC listener without always calling `Node.Shutdown()`, so the process could appear “stuck” with VPN still active.
4. Daemon subprocesses started via `mynet up` did not use a new session (`setsid`), which can worsen signal delivery edge cases on Unix.

**Mitigations in V0.2.7+:**

- `down` vs `quit` semantics separated (`Node.Down` vs `Node.Shutdown`).
- `quit` runs full shutdown then closes the listener; CLI waits up to `DefaultQuitWait` (10s) for PID exit.
- `mynet up` starts `_daemon` with `setsid` on Unix.
- No broad `pkill`; only verified PID files and IPC.

If `down` is killed, retry `mynet down` or use `mynet daemon stop` after verifying PID: `cat ~/.mynet/mynetd.pid` and `ps -p <pid>`.

**Note:** On macOS, Unix domain socket paths must stay short (historically ~104 bytes). Integration tests use `/tmp/mynetd*` state dirs to avoid `bind: invalid argument` when paths grow long.
