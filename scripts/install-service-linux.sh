#!/usr/bin/env bash
# Prepare/install MyNet systemd service (requires sudo for live install).
set -euo pipefail
REPO="${MYNET_REPO:-$HOME/mynet}"
BIN="${MYNET_BIN:-$REPO/bin/mynet-v030-candidate}"
DRY=0
for a in "$@"; do
  case "$a" in
    --dry-run) DRY=1 ;;
    -h|--help) echo "Usage: $0 [--dry-run]"; exit 0 ;;
    *) echo "unknown: $a" >&2; exit 2 ;;
  esac
done
cd "$REPO"
[[ -x "$BIN" ]] || { echo "missing binary: $BIN" >&2; exit 1; }
ARGS=(service install --binary "$BIN" --user "${USER}")
[[ "$DRY" -eq 1 ]] && ARGS+=(--dry-run)
if [[ "$DRY" -eq 1 ]]; then
  "$BIN" "${ARGS[@]}"
else
  echo "Run: sudo -E $BIN ${ARGS[*]}"
  echo "Then: sudo $BIN service start"
fi
