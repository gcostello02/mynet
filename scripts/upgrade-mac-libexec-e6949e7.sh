#!/usr/bin/env bash
# Prints a900233 → e6949e7 libexec upgrade steps (does not run sudo).
set -euo pipefail
REPO="${MYNET_REPO:-$HOME/Documents/GitHub/mynet}"
CAND="${MYNET_BIN:-$REPO/bin/mynet-v030-candidate}"
DOC="$REPO/docs/mac-libexec-upgrade-e6949e7.md"
cat <<EOF
=== MyNet libexec upgrade a900233 → e6949e7 ===

Full procedure: $DOC

Preflight:  go version -m $CAND | grep vcs.revision
Stage:      sudo $CAND service upgrade --candidate $CAND --stage-only
Stop:       sudo $CAND service stop
Commit:     sudo $CAND service upgrade --commit --live /usr/local/libexec/mynet/mynet
Start:      sudo $CAND service start

Early backup (optional): sudo cp -a /usr/local/libexec/mynet/mynet /usr/local/libexec/mynet/mynet.a900233.rollback
EOF
