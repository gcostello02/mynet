#!/usr/bin/env bash
# Operator steps: mini PC supervised V0.3.0 binary → ICE admission fix (no auto sudo).
set -euo pipefail
REPO="${MYNET_REPO:-$HOME/mynet}"
LIVE="${REPO}/bin/mynet"
CAND="${REPO}/bin/mynet-v030-candidate"
BACKUP="${REPO}/bin/mynet.v030-pre-ice-fix"
STATE="${MYNET_STATE_DIR:-${HOME}/.mynet}"
OWNER="${MYNET_OWNER_USER:-${USER}}"
EXPECTED_REV="${MYNET_ICE_FIX_REV:-}"

cat <<EOF
=== MyNet V0.3.0 ICE session fix — mini PC (systemd supervised) ===

Prereq: ${CAND} built with vcs.modified=false (scripts/build-v030-candidate.sh).
LAN SSH recovery: ssh youruser@192.0.2.10 (independent of mesh VPN).

0. Verify candidate (no sudo):
   go version -m ${CAND} | grep vcs
   ${EXPECTED_REV:+   # Expect vcs.revision=${EXPECTED_REV}}

1. Brief outage — stop supervised daemon:
   sudo ${LIVE} service stop
   systemctl is-active mynet-daemon    # inactive

2. Backup live binary (rollback artifact):
   cp -a ${LIVE} ${BACKUP}

3. Install candidate:
   cp -a ${CAND} ${LIVE}
   go version -m ${LIVE} | grep vcs

4. Start (systemd unit unchanged unless IPC caps were updated separately):
   sudo ${LIVE} service start
   sleep 15
   ${LIVE} status
   ${LIVE} diagnose --peer macbook | head -30

5. Observe ICE recovery after restart (optional second checkpoint):
   sudo ${LIVE} service stop && sudo ${LIVE} service start
   # Expect VPN up without ~30s credential-timeout streak

--- Rollback (mini only) ---
   sudo ${LIVE} service stop
   cp -a ${BACKUP} ${LIVE}
   sudo ${LIVE} service start
   ${LIVE} status

Do NOT: systemctl enable, modify coordinator/TURN, or delete ${STATE}.
EOF
