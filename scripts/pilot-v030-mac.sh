#!/usr/bin/env bash
# MyNet V0.3.0 controlled pilot — macOS (prints steps; does NOT run sudo automatically).
set -euo pipefail
REPO="${MYNET_REPO:-$HOME/Documents/GitHub/mynet}"
LIVE="${REPO}/bin/mynet"
CAND="${REPO}/bin/mynet-v030-candidate"
BACKUP="${REPO}/bin/mynet.v0210-good"

cat <<EOF
=== MyNet V0.3.0 pilot (MacBook) — run ONLY after Linux pilot passes ===

1. Stop manual V0.2.10:
   sudo -E ${LIVE} down
   sudo -E ${LIVE} daemon stop

2. Backup:
   cp -a ${LIVE} ${BACKUP}
   cp -a ${CAND} ${LIVE}

3. Install launchd:
   ${LIVE} service install --binary ${LIVE} --user ${USER} --dry-run
   sudo -E ${LIVE} service install --binary ${LIVE} --user ${USER}

4. Start:
   sudo ${LIVE} service start
   ${LIVE} service status

Rollback:
   sudo ${LIVE} service stop
   sudo launchctl bootout system /Library/LaunchDaemons/com.mynet.daemon.plist 2>/dev/null || true
   cp -a ${BACKUP} ${LIVE}
   sudo -E env MYNET_ENABLE_PPROF=1 ${LIVE} up
EOF
