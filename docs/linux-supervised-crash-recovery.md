# Linux supervised daemon — crash recovery validation (operator)

**Purpose:** Verify systemd restarts the MyNet client after an unexpected process exit, without touching the coordinator, identities, or LAN SSH.

**Prerequisites**

- Mini PC: `mynet-daemon` unit installed, **boot autostart disabled** (as today).
- MacBook: manual daemon running; mesh was healthy before the test.
- LAN SSH: `ssh youruser@10.0.0.50` (out-of-band).
- Record before state: `mynet service status`, `mynet diagnose --peer macbook`, `systemctl show mynet-daemon -p NRestarts,MainPID`.

**Procedure (operator only — not automated)**

1. On the mini, confirm VPN + IPC: `mynet status`, `ping -c 3 172.31.240.1`.
2. Note the supervised PID: `systemctl show mynet-daemon -p MainPID --value`.
3. **Simulate crash** (supervised process only): as root, `kill -SEGV <MainPID>` or `kill -9 <MainPID>` on the `mynet _daemon` process only — **do not** stop the unit with `systemctl stop`.
4. Within ~10s: `systemctl is-active mynet-daemon` → `active`; `NRestarts` incremented by 1.
5. As user `youruser`: `mynet status` (IPC), `ls -l ~/.mynet/mynetd.sock` (owner `youruser`, mode `600`).
6. `mynet diagnose --peer macbook` — expect new ICE session ID, Direct P2P, WireGuard ready within normal recovery window (no 30s credential timeout).
7. On Mac: `mynet diagnose --peer minipc` — **same ICE session ID** as mini; `ping -c 5 172.31.240.2`.
8. Confirm LAN SSH still works; coordinator unchanged (`curl -sf https://mesh.example.com/healthz`).

**Pass criteria**

- systemd restarts daemon (`Restart=on-failure`).
- Identity and `~/.mynet` intact; user IPC works after restart.
- VPN reconnects; both peers share one ICE session.
- No coordinator/TURN/SSH route changes required.

**Rollback**

- If the unit enters a failed state: operator restores last known-good binary from `~/mynet/bin/mynet.*-good` and `sudo mynet service restart` (outside this doc’s scope).
