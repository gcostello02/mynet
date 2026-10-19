#!/usr/bin/env bash
# Prints libexec migration steps — does NOT run sudo or stop services.
set -euo pipefail
REPO="${MYNET_REPO:-$HOME/Documents/GitHub/mynet}"
CAND="${MYNET_BIN:-$REPO/bin/mynet-v030-candidate}"
DOC="$REPO/docs/mac-launchd-libexec-migration.md"
cat <<EOF
=== MyNet macOS libexec migration (operator-run) ===

Full doc: $DOC

Candidate CLI: $CAND
Target daemon:  /usr/local/libexec/mynet/mynet

1. $CAND service audit
2. Backup plist + binary (see doc)
3. sudo $CAND service stop
4. sudo -E $CAND service install --binary $CAND --user $USER --system-binary
5. sudo $CAND service start && $CAND service audit

Dev repo CLI stays at $CAND; launchd runs libexec only.
EOF
