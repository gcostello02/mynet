# macOS LaunchDaemon pilot (V0.3.0)

Operator-run procedure for migrating the MacBook from a **manual** `mynet _daemon` to **launchd** supervision. Do not run on the coordinator mini PC.

**Safety:** This document is read-only until you execute each stage. The agent must not run `sudo`, install launchd, or stop live daemons during preparation.

## Terminology (macOS launchd)

| Term | Meaning |
|------|---------|
| **Plist installed** | `/Library/LaunchDaemons/com.mynet.daemon.plist` exists on disk |
| **Loaded** | Job registered in launchd (`launchctl print system/com.mynet.daemon` succeeds) |
| **Running** | Loaded job has an active process (`state = running` or `pid =` in print output) |
| **Boot autostart** | Plist under `/Library/LaunchDaemons` is loaded automatically on **reboot** (no separate `enable` like systemd). Avoid reboot until pilot stages pass. |
| **bootstrap** | `launchctl bootstrap system <plist>` — loads job for current boot; `RunAtLoad` starts the daemon |
| **bootout** | `launchctl bootout system <plist>` — unloads job; plist file may remain |

`mynet service install` writes the plist only; it does **not** call `bootstrap`. `mynet service start` runs `bootstrap`.

## Preconditions

- Mac on candidate or live V0.3.0+ (`mynet` with service commands).
- Identity at `~/.mynet` (unchanged by install).
- Binary path for install must be **absolute** (e.g. `/Users/you/Documents/GitHub/mynet/bin/mynet-v030-candidate`).
- Coordinator, Tailscale, SSH, and firewall are **out of scope** — do not change them.

---

## Stage A — Stop manual daemon; install plist (not loaded)

**Goal:** No MyNet process; launchd plist on disk; launchd not loaded.

```bash
export MYNET_BIN="$HOME/Documents/GitHub/mynet/bin/mynet-v030-candidate"  # or live bin
mynet status
mynet down
mynet daemon stop
# confirm no listener
test ! -S "$HOME/.mynet/mynetd.sock" && echo "socket gone OK"

"$MYNET_BIN" service install --dry-run --binary "$MYNET_BIN" --user "$USER"
sudo -E "$MYNET_BIN" service install --binary "$MYNET_BIN" --user "$USER"
```

**Expected**

- Dry-run prints `launchd`, paths, owner.
- Install succeeds; message says plist written, **not** loaded.
- `mynet service status`: `system unit: present=true`, `supervisor run: false`, daemon IPC not reachable (manual stopped).
- `plutil -lint /Library/LaunchDaemons/com.mynet.daemon.plist` → `OK`.

**Failure**

- `daemon still running` → repeat `mynet daemon stop`; check `pgrep -fl mynet`.
- Install as root without `--user` → use `--user "$USER"` or `sudo -E`.

**Rollback (Stage A)**

```bash
sudo "$MYNET_BIN" service uninstall
# manual again:
sudo -E env MYNET_ENABLE_PPROF=1 "$MYNET_BIN" up   # or your prior manual flow
```

---

## Stage B — Verify registration without starting

```bash
plutil -lint /Library/LaunchDaemons/com.mynet.daemon.plist
ls -la /Library/LaunchDaemons/com.mynet.daemon.plist
grep -E 'MYNET_STATE_DIR|MYNET_OWNER_USER|MYNET_SUPERVISED|MYNET_RUN_VPN_AT_START' /Library/LaunchDaemons/com.mynet.daemon.plist
launchctl print system/com.mynet.daemon 2>&1 | head -5   # expect failure / not found
mynet service status
```

**Expected**

- Plist mode `644`, root-owned.
- Env keys point at your `~/.mynet` and username.
- `launchctl print` fails (not loaded).
- `mynet up` refuses detached spawn: *service is installed but daemon is not reachable*.

**Failure**

- Accidental `service start` → `sudo mynet service stop` (bootout).

**Rollback:** Same as Stage A uninstall.

---

## Stage C — Start supervised daemon; verify VPN

```bash
sudo "$MYNET_BIN" service start
sleep 3
mynet service status
mynet status
# optional: ping peer mesh IP, check WireGuard / coordination
sudo tail -n 50 /var/log/mynet/daemon.log
```

**Expected**

- `launchctl print system/com.mynet.daemon` shows `state = running`.
- `mynet service status`: supervisor run true, daemon IPC true, VPN data path true.
- Log shows supervised VPN up, coordination connected.

**Failure**

- Immediate exit / restart loop in log → check state dir permissions, `MYNET_OWNER_USER`, identity paths.
- VPN up error within 2 min → supervised process exits with code 1; launchd restarts after `ThrottleInterval` (10s).

**Rollback**

```bash
sudo "$MYNET_BIN" service stop
sudo "$MYNET_BIN" service uninstall
# restore manual daemon if needed
```

---

## Stage D — Graceful supervised stop / start

```bash
sudo "$MYNET_BIN" service stop
launchctl print system/com.mynet.daemon 2>&1 | head -3
mynet service status   # not reachable
sudo "$MYNET_BIN" service start
mynet service status   # reachable again
```

**Expected**

- `service stop` → `bootout`; VPN and daemon down.
- `service start` → `bootstrap`; VPN returns via `MYNET_RUN_VPN_AT_START`.

**Also test (while running)**

- `mynet down` → VPN down, **daemon still running** (IPC reachable).
- `mynet up` → VPN up via IPC (no second daemon).

**Failure**

- `mynet daemon stop` while supervised → IPC quit, exit 0; launchd should **not** restart (`KeepAlive` / `SuccessfulExit` false).

---

## Stage E — Crash recovery

While supervised and VPN up:

```bash
sudo pkill -9 -f 'mynet _daemon'   # simulate crash; use with care
sleep 15
mynet service status
sudo tail -n 30 /var/log/mynet/daemon.log
```

**Expected**

- launchd restarts daemon within ~10s throttle.
- New PID; VPN and peers recover (may take ICE renegotiation).

**Failure**

- Stale `utun` / routes → inspect `ifconfig`, `netstat -rn`; may need supervised stop and manual interface cleanup (document before doing).

**Rollback:** Stage C rollback.

---

## Stage F — Boot persistence (optional, separate)

Install already places plist in `/Library/LaunchDaemons`. After successful Stage C–E:

- **Without reboot:** loaded state is whatever `bootstrap` did; `bootout` unloads until next reboot or `start` again.
- **After reboot:** launchd loads all valid plists in `/Library/LaunchDaemons` and honors `RunAtLoad`.

Schedule a maintenance window for reboot validation. Do not reboot during initial A–E on the same day as coordinator changes.

There is no `systemctl enable` equivalent; persistence is **plist on disk** + not using `launchctl disable`.

---

## CLI behavior with plist installed

| Command | Plist only (not loaded) | Loaded / running |
|---------|-------------------------|------------------|
| `mynet up` | Error (no detached spawn) | IPC `up` if daemon up |
| `mynet down` | No daemon | VPN down, daemon stays |
| `mynet daemon stop` | No daemon | IPC quit (prefer `service stop`) |

---

## Files touched by install

| Path | Purpose |
|------|---------|
| `/Library/LaunchDaemons/com.mynet.daemon.plist` | launchd job |
| `/Library/Preferences/com.mynet.daemon.env` | Metadata copy (plist uses inline env) |
| `/var/log/mynet/daemon.log` | stdout/stderr |
| `~/.mynet/service.json` | User-readable install metadata (mode 0600, owner you) |

Identity, keys, and `wg.json` remain under `~/.mynet`.
