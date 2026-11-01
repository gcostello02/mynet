# Mac libexec upgrade: a900233 → e6949e7

Operator-run only. Replaces `/usr/local/libexec/mynet/mynet` only; plist, identity, and Linux mini unchanged.

## Recommendation

Use **`mynet service upgrade`** (stage → stop → commit), not **`service install`**, when:

- Plist already points at libexec and env is correct
- You only need a binary swap

Use **`service install --system-binary`** only if plist/env/service.json must be regenerated.

`--live` defaults from `~/.mynet/service.json` to `/usr/local/libexec/mynet/mynet`.

## Variables

```bash
export REPO="$HOME/Documents/GitHub/mynet"
export CAND="$REPO/bin/mynet-v030-candidate"
export LIVE="/usr/local/libexec/mynet/mynet"
export BACKUP="/usr/local/libexec/mynet/mynet.a900233.rollback"
```

## 1. Preflight (no sudo)

```bash
go version -m "$CAND" | grep vcs.revision   # expect e6949e7…
file "$CAND"                                 # Mach-O arm64
go version -m "$LIVE" | grep vcs.revision    # expect a900233… (live)

$CAND service audit
$CAND service status
$CAND diagnose --peer minipc
launchctl print system/com.mynet.daemon | grep -E 'program|pid|state'
pgrep -fl 'mynet _daemon'
```

## 2. Rollback backup (optional early; does not modify live)

```bash
sudo cp -a "$LIVE" "$BACKUP"
sudo shasum -a 256 "$LIVE" "$BACKUP"
```

## 3. Stage new binary (sudo; daemon may keep running)

```bash
sudo "$CAND" service upgrade \
  --candidate "$CAND" \
  --live "$LIVE" \
  --backup "$BACKUP" \
  --stage-only
ls -la "${LIVE}.next"
go version -m "${LIVE}.next" | grep vcs.revision   # e6949e7
```

## 4. Stop supervised daemon

```bash
sudo "$CAND" service stop
mynet service status    # IPC false
pgrep -fl 'mynet _daemon' || echo "no daemon OK"
```

## 5. Commit swap

```bash
sudo "$CAND" service upgrade --commit --live "$LIVE" --backup "$BACKUP"
go version -m "$LIVE" | grep vcs.revision   # e6949e7
ls -la "$LIVE" "$BACKUP"
```

`CommitSwap` backs up live to `$BACKUP` if not already done, renames `.next` → live, `chown root:wheel` on libexec.

## 6. Start and verify

```bash
sudo "$CAND" service start
sleep 5
$CAND service status
$CAND diagnose --peer minipc
sudo tail -n 30 /var/log/mynet/daemon.log
```

Expect new log lines: `daemon IPC listening`, `supervised VPN up starting`, `supervised VPN up complete`.

```bash
ping -c 3 172.31.240.2
ssh youruser@192.0.2.10 true
```

## Rollback

```bash
sudo "$CAND" service stop
sudo cp -a "$BACKUP" "$LIVE"
sudo chown root:wheel "$LIVE"
sudo "$CAND" service start
go version -m "$LIVE" | grep vcs.revision   # a900233
```

## Alternate: install (rewrites plist)

Only if upgrade commit fails:

```bash
sudo "$CAND" service stop
sudo -E "$CAND" service install --binary "$CAND" --user "$USER" --system-binary
sudo "$CAND" service start
```
