#!/usr/bin/env bash
# Hot-swap V0.3.0 binary after IPC ownership fix (supervised daemon must be restarted by operator).
set -euo pipefail
REPO="${MYNET_REPO:-$HOME/mynet}"
LIVE="${REPO}/bin/mynet"
CAND="${REPO}/bin/mynet-v030-candidate"
BACKUP="${REPO}/bin/mynet.v030-pre-ipc-fix"
STATE="${MYNET_STATE_DIR:-${HOME}/.mynet}"
OWNER="${MYNET_OWNER_USER:-${USER}}"

cat <<EOF
=== MyNet V0.3.0 IPC ownership fix — operator steps (sudo required) ===

1. Stop supervised daemon (brief VPN outage):
   sudo ${LIVE} service stop
   systemctl is-active mynet-daemon    # inactive
   pgrep -af '[m]ynet.*_daemon' || echo "no daemon OK"

2. Backup current binary:
   cp -a ${LIVE} ${BACKUP}

3. Install corrected candidate:
   cp -a ${CAND} ${LIVE}
   go version -m ${LIVE} | grep vcs

4. Refresh systemd unit (adds CAP_CHOWN) — still no enable:
   sudo -E ${LIVE} service install --binary ${LIVE} --state-dir ${STATE} --user ${OWNER}
   sudo grep CAP_CHOWN /etc/systemd/system/mynet-daemon.service

5. Start and verify IPC as normal user:
   sudo ${LIVE} service start
   sleep 15
   ls -l ${STATE}/mynetd.sock ${STATE}/mynetd.pid
   ${LIVE} status
   sudo ${LIVE} status
   ping -c 3 172.31.240.1

Rollback:
   sudo ${LIVE} service stop
   cp -a ${BACKUP} ${LIVE}
   sudo -E ${LIVE} service install --binary ${LIVE} --state-dir ${STATE} --user ${OWNER}
   sudo ${LIVE} service start
EOF
