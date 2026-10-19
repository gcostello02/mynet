#!/usr/bin/env bash
# MyNet V0.3.0 — macOS LaunchDaemon pilot (prints stages; does NOT run sudo).
set -euo pipefail
REPO="${MYNET_REPO:-$HOME/Documents/GitHub/mynet}"
CAND="${MYNET_BIN:-$REPO/bin/mynet-v030-candidate}"
DOC="$REPO/docs/mac-launchd-pilot-v030.md"

if [[ ! -f "$DOC" ]]; then
  echo "missing $DOC" >&2
  exit 1
fi

cat <<EOF
=== MyNet V0.3.0 macOS LaunchDaemon pilot ===

Full procedure: $DOC

Binary: $CAND
User:   $USER

Stage A (stop manual, install plist — NOT started):
  mynet down && mynet daemon stop
  $CAND service install --dry-run --binary $CAND --user $USER
  sudo -E $CAND service install --binary $CAND --user $USER
  plutil -lint /Library/LaunchDaemons/com.mynet.daemon.plist

Stage B: verify launchctl print fails (not loaded); mynet service status

Stage C: sudo $CAND service start  &&  mynet service status

Rollback: sudo $CAND service stop && sudo $CAND service uninstall

Do NOT reboot until stages C–E pass. Do NOT touch coordinator or mynet-server on mini.
EOF
