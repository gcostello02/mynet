#!/usr/bin/env bash
# Operator steps: MacBook manual daemon V0.2.10 → V0.3.0+ ICE fix (no launchd).
set -euo pipefail
REPO="${MYNET_REPO:-$HOME/Documents/GitHub/mynet}"
LIVE="${REPO}/bin/mynet"
CAND="${REPO}/bin/mynet-v030-candidate"
BACKUP="${REPO}/bin/mynet.v0210-good"
EXPECTED_REV="${MYNET_ICE_FIX_REV:-}"

cat <<EOF
=== MyNet ICE fix — MacBook manual daemon (no launchd) ===

Prereq: ${CAND} built on darwin/arm64 or amd64 with vcs.modified=false.
LAN SSH to mini remains: ssh youruser@192.0.2.10

0. Verify candidate:
   go version -m ${CAND} | grep vcs
   ${EXPECTED_REV:+   # Expect vcs.revision=${EXPECTED_REV}}

1. Stop manual VPN + daemon (brief outage):
   sudo -E ${LIVE} down
   sudo -E ${LIVE} daemon stop
   pgrep -fl 'mynet _daemon' || true

2. Backup V0.2.10 binary:
   cp -a ${LIVE} ${BACKUP}

3. Install candidate (does not overwrite until you copy):
   cp -a ${CAND} ${LIVE}
   go version -m ${LIVE} | grep vcs

4. Start manual daemon (same workflow as today):
   sudo -E env MYNET_ENABLE_PPROF=1 ${LIVE} up
   ${LIVE} status
   ping -c 3 172.31.240.1

--- Rollback (Mac only) ---
   sudo -E ${LIVE} down
   sudo -E ${LIVE} daemon stop
   cp -a ${BACKUP} ${LIVE}
   sudo -E env MYNET_ENABLE_PPROF=1 ${LIVE} up

Coordinator, identities, and ~/.mynet are untouched.
Deploy mini first; Mac upgrade removes already_active stale-state on the responder.
EOF
