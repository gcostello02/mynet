#!/usr/bin/env bash
# MyNet V0.3.0 controlled pilot — Linux mini PC (prints steps; does NOT run sudo automatically).
set -euo pipefail
REPO="${MYNET_REPO:-$HOME/mynet}"
LIVE="${REPO}/bin/mynet"
CAND="${REPO}/bin/mynet-v030-candidate"
BACKUP="${REPO}/bin/mynet.v0210-good"
STATE="${MYNET_STATE_DIR:-${HOME}/.mynet}"
OWNER="${MYNET_OWNER_USER:-${USER}}"
AUDIT_REV="${MYNET_AUDIT_REV:-a4b6b3316af2407e8cdc59c5e02be9690c84ebf8}"

rollback_supervision() {
  # Safe even if install never completed or only partially wrote unit files.
  if [[ -x "${LIVE}" ]]; then
    sudo "${LIVE}" service stop 2>/dev/null || true
    sudo -E "${LIVE}" service uninstall 2>/dev/null || true
  fi
  if systemctl list-unit-files mynet-daemon.service &>/dev/null; then
    sudo systemctl disable --now mynet-daemon.service 2>/dev/null || true
  fi
  sudo rm -f /etc/systemd/system/mynet-daemon.service /etc/mynet/client.env 2>/dev/null || true
  sudo systemctl daemon-reload 2>/dev/null || true
}

cat <<EOF
=== MyNet V0.3.0 pilot (mini PC) — interactive sudo required ===

Audit commit: ${AUDIT_REV}
State dir:    ${STATE}
Owner user:   ${OWNER}

0. Candidate metadata (before any sudo):
   go version -m ${CAND} | grep vcs
   # Expect: vcs.revision=${AUDIT_REV} and vcs.modified=false

1. Verify health (no changes):
   ${LIVE} status
   curl -sf https://mesh.example.com/healthz

2. Stop manual V0.2.10 client (VPN + daemon) — required before service install:
   sudo -E ${LIVE} down
   sudo -E ${LIVE} daemon stop
   pgrep -fl 'mynet _daemon' || true

3. Backup binary:
   cp -a ${LIVE} ${BACKUP}

4. Install V0.3.0 binary:
   cp -a ${CAND} ${LIVE}
   go version -m ${LIVE} | grep vcs

5. Register systemd only (does NOT start; does NOT enable boot):
   ${LIVE} service install --binary ${LIVE} --state-dir ${STATE} --user ${OWNER} --dry-run
   sudo -E ${LIVE} service install --binary ${LIVE} --state-dir ${STATE} --user ${OWNER}
   systemctl is-active mynet-daemon    # expect: inactive
   # Do NOT run: sudo systemctl enable mynet-daemon  (until separately approved)

6. Start supervised daemon (installation checkpoint ends before this unless you proceed):
   sudo ${LIVE} service start
   sleep 15
   ${LIVE} service status
   ${LIVE} status
   ${LIVE} diagnose --peer macbook | head -40

7. Connectivity (mesh only; LAN SSH on 10.0.0.50 is out of band):
   ping -c 5 172.31.240.1

--- Rollback to manual V0.2.10 (run if pilot fails at any step after step 2) ---

  # 1) Remove supervision first (partial install safe):
  $(declare -f rollback_supervision)
  rollback_supervision

  # 2) Restore known-good binary and manual daemon:
  cp -a ${BACKUP} ${LIVE}
  sudo -E env MYNET_ENABLE_PPROF=1 ${LIVE} up
  ${LIVE} status

Identity and ${STATE} are never deleted by service install/uninstall.

Coordinator / mynet-server: DO NOT MODIFY.
EOF
