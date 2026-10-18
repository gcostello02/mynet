#!/usr/bin/env bash
# MyNet V0.2.9 controlled rollout — Ubuntu mini PC (client daemon only).
#   bash scripts/deploy-v029-mini.sh              # live rollout (sudo)
#   bash scripts/deploy-v029-mini.sh --preflight  # checks only, no sudo/stop/install
set -euo pipefail

REPO="${MYNET_REPO:-$HOME/mynet}"
CANDIDATE="bin/mynet-v029-candidate"
BACKUP="bin/mynet.v028-good"
LIVE="bin/mynet"
PROF_ENV=(MYNET_ENABLE_PPROF=1)
PREFLIGHT=0

for arg in "$@"; do
  case "$arg" in
    --preflight|--dry-run) PREFLIGHT=1 ;;
    -h|--help)
      echo "Usage: $0 [--preflight|--dry-run]"
      echo "  --preflight  Verify host, git, binaries, daemon identity; no deployment."
      exit 0
      ;;
    *) echo "Unknown option: $arg" >&2; exit 2 ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=deploy-v029-lib.sh
source "$SCRIPT_DIR/deploy-v029-lib.sh"

cd "$REPO" || die "repo missing: $REPO"

run_checks() {
  info "Host check (expect Linux amd64 mini PC)"
  [[ "$(uname -s)" == "Linux" ]] || die "not Linux"
  [[ "$(uname -m)" == "x86_64" ]] || die "not amd64"
  HOST="$(hostname -s 2>/dev/null || hostname)"
  [[ "$HOST" == "minipc" ]] || echo "WARN: hostname is '$HOST' (expected minipc); continuing if repo path is correct"

  verify_git_contains_implementation

  [[ -x "$BACKUP" ]] || die "missing V0.2.8 backup $BACKUP (cp -a $LIVE $BACKUP)"
  [[ -x "$CANDIDATE" ]] || die "missing candidate $CANDIDATE (go build -o $CANDIDATE ./cmd/mynet)"

  verify_backup_binary "$BACKUP"
  verify_candidate_binary "$CANDIDATE"
  report_live_binary "$LIVE"
  preflight_daemon_identity "$REPO" "$LIVE"

  if ip link show mynet0 &>/dev/null; then
    info "mynet0 present (expected while daemon runs; live deploy requires it gone after stop)"
  else
    info "mynet0 not present"
  fi
}

if [[ "$PREFLIGHT" -eq 1 ]]; then
  run_checks
  info "Preflight OK on $(hostname -s 2>/dev/null || hostname). Deployment was NOT performed."
  exit 0
fi

run_checks

info "Identify MyNet client daemon (exact path, no broad pkill)"
find_mynet_daemon_pids "$REPO" "$LIVE"
if [[ ${#PIDS[@]} -gt 1 ]]; then
  die "multiple mynet _daemon PIDs: ${PIDS[*]} — resolve manually"
fi
if [[ ${#PIDS[@]} -eq 1 ]]; then
  PID="${PIDS[0]}"
  info "Stopping daemon PID=$PID via mynet daemon stop"
  ps -p "$PID" -o pid,user,cmd || true
  sudo -E "./$LIVE" daemon stop
  sleep 2
  if kill -0 "$PID" 2>/dev/null; then
    die "daemon PID $PID still running after daemon stop"
  fi
else
  info "No running mynet _daemon found; continuing install"
fi

if ip link show mynet0 &>/dev/null; then
  die "mynet0 still present after stop; investigate before install"
fi

info "Install V0.2.9 candidate atomically"
cp -a "$CANDIDATE" "${LIVE}.new"
chmod +x "${LIVE}.new"
verify_candidate_binary "${LIVE}.new"
mv "${LIVE}.new" "$LIVE"

info "Start VPN + daemon (profiling enabled)"
sudo -E env "${PROF_ENV[@]}" "./$LIVE" up
sleep 12

info "Post-start verification"
"./$LIVE" status
"./$LIVE" diagnose --peer macbook | head -40

if ! "./$LIVE" status | grep -q "WireGuard: Active"; then
  die "WireGuard not active after up"
fi
if ! "./$LIVE" status | grep -q "Transport ready"; then
  echo "WARN: peer not transport ready yet; allow ICE window and re-check diagnose"
fi

info "LAN SSH recovery path (optional)"
ping -c 2 -W 2 10.0.0.5 >/dev/null 2>&1 && info "LAN ping to Mac OK" || echo "WARN: LAN ping to 10.0.0.5 failed"

info "Done. Rollback: sudo -E ./$LIVE daemon stop; cp -a $BACKUP $LIVE; sudo -E env ${PROF_ENV[*]} ./$LIVE up"
