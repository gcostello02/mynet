# V0.3.0 acceptance tests — macOS crash recovery & reboot (operator-run)

Do not execute during automated agent sessions. Coordinator (`mynet-server` on mini) must stay running.

## A. Mac libexec crash recovery (no reboot)

**Pre:** `mynet service status` healthy; `launchctl print system/com.mynet.daemon` shows `program = /usr/local/libexec/mynet/mynet`; single `pgrep -fl 'mynet _daemon'`.

1. Record PID: `launchctl print system/com.mynet.daemon | grep pid`
2. Terminate supervised process only:
   ```bash
   sudo pkill -9 -f '/usr/local/libexec/mynet/mynet _daemon'
   ```
3. Within 15s: `launchctl print system/com.mynet.daemon` → new PID, `runs` unchanged or +0 (same job), no crash loop (`ThrottleInterval` 10s).
4. Within 60s:
   - `mynet service status` → IPC true, coordination Connected, VPN data path true
   - `mynet diagnose --peer minipc` → VPN data path ready
   - `ping -c 3 172.31.240.2`
   - `ssh youruser@192.0.2.10 true` (LAN, not mesh-only)
5. `pgrep -fl mynet` → exactly one `_daemon` from libexec path.

**Fail if:** repeated PIDs cycling (crash loop), second manual `_daemon`, identity changed, or LAN SSH broken.

**Rollback:** none required if recovery succeeds; if loop, `sudo mynet service stop` and investigate `/var/log/mynet/daemon.log`.

---

## B. macOS reboot validation

**Pre:** note `Device: macbook`, VPN IP `172.31.240.1`, libexec binary revision (`go version -m /usr/local/libexec/mynet/mynet`).

1. Schedule window; ensure mini coordinator healthy.
2. Reboot MacBook (operator action).
3. **Before login** (if console access): optional `launchctl print system/com.mynet.daemon` from recovery/SSH if available — LaunchDaemons load at boot without GUI login.
4. After boot (login not required for daemon): from another machine or after login:
   ```bash
   launchctl print system/com.mynet.daemon
   ls -la /usr/local/libexec/mynet/mynet
   mynet service status
   mynet diagnose --peer minipc
   ```
5. Validate mesh ping and LAN SSH as in A.

**Pass:** libexec binary still root-owned `0755`; identity unchanged; VPN recovers without manual `mynet up`.

---

## C. Linux mini reboot validation

**Pre:** `systemctl is-enabled mynet-daemon`; coordinator separate.

1. Reboot mini (`ssh youruser@192.0.2.10` after boot).
2. On mini:
   ```bash
   systemctl is-active mynet-daemon
   mynet service status
   mynet diagnose --peer macbook
   ```
3. Confirm `https://mesh.example.com` health (from Mac).
4. From Mac: ping `172.31.240.2`, mesh SSH if used.

**Pass:** systemd autostart; coordinator never stopped; MyNet peers reconnect without manual intervention on either side.
