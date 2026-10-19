# macOS LaunchDaemon — libexec binary migration

Move the supervised executable from a user-writable Git tree to:

`/usr/local/libexec/mynet/mynet`

Identity (`~/.mynet`), plist label (`com.mynet.daemon`), and coordinator are unchanged.

**Do not run this document automatically.** Execute stages manually during a maintenance window.

## Current risk (pilot layout)

| Item | Typical pilot state | Risk |
|------|---------------------|------|
| `ProgramArguments` binary | `~/Documents/GitHub/mynet/bin/mynet-v030-candidate` | **Critical:** owner can replace binary launchd runs as root |
| Parent dirs | `~/Documents/...` user-owned, mode `755` | Writable by login user |
| Plist | `/Library/LaunchDaemons/com.mynet.daemon.plist` `644` root | OK if not world-writable |
| State `~/.mynet` | `700` user, IPC `0600` chowned to owner | OK; not used as executable |
| Logs | `/var/log/mynet/daemon.log` | Root-created; append by daemon |

CLI from the dev repo remains normal: `~/.../bin/mynet` talks to the supervised daemon via IPC.

## Pre-migration audit (no sudo)

```bash
DEV=~/Documents/GitHub/mynet/bin/mynet-v030-candidate
$DEV service audit
plutil -lint /Library/LaunchDaemons/com.mynet.daemon.plist
grep -A2 ProgramArguments /Library/LaunchDaemons/com.mynet.daemon.plist
```

Expect `service audit` **critical** on `writable tree` until migration completes.

## Stage 1 — Validate candidate

```bash
REPO=~/Documents/GitHub/mynet
CAND=$REPO/bin/mynet-v030-candidate
go version -m "$CAND" | grep vcs.revision
$CAND service install --dry-run --binary "$CAND" --user "$USER" --system-binary
```

**Expected:** `binary: /usr/local/libexec/mynet/mynet`, `candidate: …/mynet-v030-candidate`.

## Stage 2 — Backup working install

```bash
sudo cp -a /Library/LaunchDaemons/com.mynet.daemon.plist \
  /Library/LaunchDaemons/com.mynet.daemon.plist.pre-libexec
sudo cp -a "$(grep -A1 ProgramArguments /Library/LaunchDaemons/com.mynet.daemon.plist | tail -1 | sed 's/[[:space:]]*<string>//;s/<\/string>//')" \
  /usr/local/libexec/mynet/mynet.rollback 2>/dev/null || \
sudo cp -a ~/Documents/GitHub/mynet/bin/mynet-v030-candidate /usr/local/libexec/mynet/mynet.rollback
```

Adjust rollback source to match your live plist binary path.

## Stage 3 — Stop launchd (supervised only)

```bash
sudo "$CAND" service stop
launchctl print system/com.mynet.daemon 2>&1 | head -3   # should fail
mynet service status   # IPC not reachable
```

Do **not** stop Tailscale, SSH, or `mynet-server` on the mini.

## Stage 4 — Install system binary + updated plist

```bash
sudo -E "$CAND" service install --binary "$CAND" --user "$USER" --system-binary
ls -la /usr/local/libexec/mynet/mynet
plutil -lint /Library/LaunchDaemons/com.mynet.daemon.plist
grep /usr/local/libexec/mynet/mynet /Library/LaunchDaemons/com.mynet.daemon.plist
```

**Expected:** binary `755` root:wheel; plist points at libexec path.

## Stage 5 — Start and validate

```bash
sudo "$CAND" service start
mynet service status
mynet status
# mesh: ping peer, SSH over mesh, check WireGuard / ICE in status
sudo tail -n 40 /var/log/mynet/daemon.log
$CAND service audit   # no critical on writable tree
```

## Stage 6 — Crash recovery spot-check

```bash
sudo pkill -9 -f '/usr/local/libexec/mynet/mynet _daemon'
sleep 15
mynet service status
```

## Stage 7 — Future upgrades

```bash
$CAND service upgrade --candidate "$CAND" --stage-only
sudo "$CAND" service stop
# commit swap (live defaults to libexec from service.json):
sudo "$CAND" service upgrade --candidate "$CAND"   # stage + instructions, or use CommitSwap via script
```

Prefer: `service upgrade --stage-only`, `service stop`, then run commit documented in `mynet service upgrade` / `internal/service/upgrade.go` (`CommitSwap` requires daemon stopped).

After swap to libexec path, ownership is re-applied automatically on macOS.

## Rollback

```bash
sudo "$CAND" service stop
sudo cp -a /Library/LaunchDaemons/com.mynet.daemon.plist.pre-libexec \
  /Library/LaunchDaemons/com.mynet.daemon.plist
# restore previous binary path from backup plist, or:
sudo cp -a /usr/local/libexec/mynet/mynet.rollback /path/from/old/plist
sudo "$CAND" service start
```

Or full uninstall + manual dev-binary plist (`--no-system-binary`) only if you accept the security regression.

## Boot behavior

Plist remains under `/Library/LaunchDaemons`; reboot continues to load `com.mynet.daemon`. Unchanged from pilot.
