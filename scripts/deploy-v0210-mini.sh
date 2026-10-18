#!/usr/bin/env bash
# MyNet V0.2.10 controlled rollout — Ubuntu mini PC (client daemon only).
#   bash scripts/deploy-v0210-mini.sh              # live rollout (sudo)
#   bash scripts/deploy-v0210-mini.sh --preflight
set -euo pipefail

REPO="${MYNET_REPO:-$HOME/mynet}"
CANDIDATE="bin/mynet-v0210-candidate"
BACKUP="bin/mynet.v029-good"
LIVE="bin/mynet"
PROF_ENV=(MYNET_ENABLE_PPROF=1)
PREFLIGHT=0

for arg in "$@"; do
  case "$arg" in
    --preflight|--dry-run) PREFLIGHT=1 ;;
    -h|--help) echo "Usage: $0 [--preflight|--dry-run]"; exit 0 ;;
    *) echo "Unknown option: $arg" >&2; exit 2 ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=deploy-v0210-lib.sh
source "$SCRIPT_DIR/deploy-v0210-lib.sh"

cd "$REPO" || die "repo missing: $REPO"

run_checks() {
  info "Host check (expect Linux amd64 mini PC)"
  [[ "$(uname -s)" == "Linux" ]] || die "not Linux"
  [[ "$(uname -m)" == "x86_64" ]] || die "not amd64"
  verify_git_contains_implementation
  [[ -x "$BACKUP" ]] || die "missing V0.2.9 backup $BACKUP"
  [[ -x "$CANDIDATE" ]] || die "missing candidate $CANDIDATE"
  verify_backup_binary "$BACKUP"
  verify_candidate_binary "$CANDIDATE"
  verify_live_not_already_candidate "$LIVE"
  assert_clean_install_artifacts "$LIVE"
  report_live_binary "$LIVE"
  preflight_daemon_identity "$REPO" "$LIVE"
}

if [[ "$PREFLIGHT" -eq 1 ]]; then
  run_checks
  info "Preflight OK on $(hostname -s 2>/dev/null || hostname). Deployment was NOT performed."
  exit 0
fi

run_checks

find_mynet_daemon_pids "$REPO" "$LIVE"
if [[ ${#PIDS[@]} -gt 1 ]]; then
  die "multiple mynet _daemon PIDs: ${PIDS[*]}"
fi
if [[ ${#PIDS[@]} -eq 1 ]]; then
  PID="${PIDS[0]}"
  info "Stopping daemon PID=$PID"
  sudo -E "./$LIVE" daemon stop
  sleep 2
  kill -0 "$PID" 2>/dev/null && die "daemon still running"
  ip link show mynet0 &>/dev/null && die "mynet0 still present after stop"
else
  info "No running mynet _daemon; continuing with binary swap only"
fi

assert_dataplane_quiescent_for_install

cp -a "$CANDIDATE" "${LIVE}.new"
chmod +x "${LIVE}.new"
verify_candidate_binary "${LIVE}.new"
mv "${LIVE}.new" "$LIVE"

sudo -E env "${PROF_ENV[@]}" "./$LIVE" up
sleep 12
"./$LIVE" status
"./$LIVE" diagnose --peer macbook | head -40
"./$LIVE" status | grep -q "WireGuard: Active" || die "WireGuard not active"

info "Done. Rollback: sudo -E ./$LIVE daemon stop; cp -a $BACKUP $LIVE; sudo -E env ${PROF_ENV[*]} ./$LIVE up"
