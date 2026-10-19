#!/usr/bin/env bash
# Prints mini-first deployment order and independent rollback paths.
set -euo pipefail
REPO_MAC="${MYNET_MAC_REPO:-$HOME/Documents/GitHub/mynet}"
REPO_LINUX="${MYNET_LINUX_REPO:-$HOME/mynet}"

cat <<EOF
=== V0.3.0 ICE fix — controlled rollout (plan only) ===

Order:
  1. Mini PC (systemd supervised): scripts/pilot-v030-linux-ice-update.sh
  2. MacBook (manual): scripts/pilot-v030-mac-manual-update.sh

Build candidates (each host, clean tree):
  MYNET_REPO=$REPO_MAC scripts/build-v030-candidate.sh
  # Linux on mini after git pull at fix commit:
  MYNET_REPO=$REPO_LINUX GOOS=linux GOARCH=amd64 scripts/build-v030-candidate.sh

Backups:
  Mini:  $REPO_LINUX/bin/mynet.v030-pre-ice-fix  (and prior bin/mynet.v0210-good from pilot)
  Mac:   $REPO_MAC/bin/mynet.v0210-good

Recovery:
  Mesh VPN down → use LAN SSH youruser@10.0.0.50
  Roll back either host independently using the BACKUP paths above

Live after pilot (reference):
  Mini systemd: V0.3.0 @ 824acf1 (IPC); target ICE fix commit after build
  Mac manual:   V0.2.10 @ eea2067 until step 2

Do not enable systemd at boot until separately approved.
EOF
