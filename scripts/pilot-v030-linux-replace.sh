#!/usr/bin/env bash
# Replace installed V0.3.0 binary + systemd unit after a fix (service must stay stopped/disabled).
set -euo pipefail
REPO="${MYNET_REPO:-$HOME/mynet}"
LIVE="${REPO}/bin/mynet"
CAND="${REPO}/bin/mynet-v030-candidate"
BACKUP_V030="${REPO}/bin/mynet.v030-pilot-broken"
STATE="${MYNET_STATE_DIR:-${HOME}/.mynet}"
OWNER="${MYNET_OWNER_USER:-${USER}}"

cat <<EOF
=== MyNet V0.3.0 Linux replacement (post-fix) — sudo required; do NOT enable or start yet ===

0. Confirm service is down (abort if active):
   systemctl is-active mynet-daemon    # expect: inactive
   systemctl is-enabled mynet-daemon   # expect: disabled
   pgrep -fl 'mynet _daemon' || echo "no daemon OK"

1. Backup current installed client binary:
   cp -a ${LIVE} ${BACKUP_V030}

2. Install corrected candidate:
   cp -a ${CAND} ${LIVE}
   go version -m ${LIVE} | grep vcs

3. Refresh systemd unit + env (does not start):
   ${LIVE} service install --binary ${LIVE} --state-dir ${STATE} --user ${OWNER} --dry-run
   sudo -E ${LIVE} service install --binary ${LIVE} --state-dir ${STATE} --user ${OWNER}
   sudo systemd-analyze verify /etc/systemd/system/mynet-daemon.service
   sudo grep -E 'StartLimit|CapabilityBounding|ExecStart' /etc/systemd/system/mynet-daemon.service
   sudo cat /etc/mynet/client.env

4. CHECKPOINT — stop before supervised start:
   systemctl is-active mynet-daemon    # still inactive
   # Next (only when approved): sudo ${LIVE} service start

Rollback to broken binary + old unit: restore ${BACKUP_V030} and re-run service install with that binary, or full pilot rollback script.
EOF
