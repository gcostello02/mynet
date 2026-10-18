#!/usr/bin/env bash
# MyNet V0.3.0 controlled pilot — Linux mini PC (prints steps; does NOT run sudo automatically).
set -euo pipefail
REPO="${MYNET_REPO:-$HOME/mynet}"
LIVE="${REPO}/bin/mynet"
CAND="${REPO}/bin/mynet-v030-candidate"
BACKUP="${REPO}/bin/mynet.v0210-good"
STATE="${HOME}/.mynet"

cat <<EOF
=== MyNet V0.3.0 pilot (mini PC) — interactive sudo required ===

1. Verify health (no changes):
   ${LIVE} status
   curl -sf https://mesh.example.com/healthz

2. Stop manual V0.2.10 client (VPN + daemon):
   sudo -E ${LIVE} down
   sudo -E ${LIVE} daemon stop
   pgrep -fl 'mynet _daemon' || true

3. Backup binary:
   cp -a ${LIVE} ${BACKUP}

4. Install V0.3.0 binary (if not already):
   cp -a ${CAND} ${LIVE}

5. Register systemd (dry-run first):
   ${LIVE} service install --binary ${LIVE} --user ${USER} --dry-run
   sudo -E ${LIVE} service install --binary ${LIVE} --user ${USER}

6. Start supervised daemon:
   sudo ${LIVE} service start
   sleep 15
   ${LIVE} service status
   ${LIVE} status
   ${LIVE} diagnose --peer macbook | head -40

7. Connectivity:
   ping -c 5 172.31.240.1
   ssh <user>@172.31.240.1 true

Rollback:
   sudo ${LIVE} service stop
   cp -a ${BACKUP} ${LIVE}
   sudo -E env MYNET_ENABLE_PPROF=1 ${LIVE} up

Coordinator/mynet-server: DO NOT MODIFY.
EOF
