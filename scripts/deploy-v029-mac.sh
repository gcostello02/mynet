#!/usr/bin/env bash
# MyNet V0.2.9 controlled rollout — macOS MacBook (client daemon only).
# Run interactively on the Mac: bash scripts/deploy-v029-mac.sh
# Deploy mini PC first and confirm health before running this script.
set -euo pipefail

REPO="${MYNET_REPO:-$HOME/Documents/GitHub/mynet}"
EXPECTED_COMMIT="21092d3aa8c54b7b5a079c62fb88d78f934adc0f"
CANDIDATE="bin/mynet-v029-candidate"
BACKUP="bin/mynet.v028-good"
LIVE="bin/mynet"
PROF_ENV=(MYNET_ENABLE_PPROF=1)

die() { echo "ERROR: $*" >&2; exit 1; }
info() { echo "==> $*"; }

info "Host check (expect macOS arm64)"
[[ "$(uname -s)" == "Darwin" ]] || die "not Darwin"
[[ "$(uname -m)" == "arm64" ]] || die "not arm64"

cd "$REPO" || die "repo missing: $REPO"

ACTUAL="$(git rev-parse HEAD)"
[[ "$ACTUAL" == "$EXPECTED_COMMIT" ]] || die "git HEAD $ACTUAL != $EXPECTED_COMMIT"

[[ -x "$BACKUP" ]] || die "missing V0.2.8 backup $BACKUP"
[[ -x "$CANDIDATE" ]] || die "missing candidate $CANDIDATE"

if ! go version -m "$BACKUP" 2>/dev/null | grep -q 'f243542'; then
  die "backup does not look like f243542"
fi
if ! go version -m "$CANDIDATE" 2>/dev/null | grep -q '21092d3'; then
  die "candidate is not 21092d3"
fi

info "Identify MyNet client daemon"
mapfile -t PIDS < <(pgrep -f "${REPO}/${LIVE} _daemon" || true)
if [[ ${#PIDS[@]} -eq 0 ]]; then
  mapfile -t PIDS < <(pgrep -f "mynet _daemon" || true)
fi
if [[ ${#PIDS[@]} -gt 1 ]]; then
  die "multiple mynet _daemon PIDs: ${PIDS[*]}"
fi
if [[ ${#PIDS[@]} -eq 1 ]]; then
  PID="${PIDS[0]}"
  info "Stopping daemon PID=$PID"
  ps -p "$PID" -o pid,user,command || true
  sudo -E "./$LIVE" daemon stop
  sleep 2
  if kill -0 "$PID" 2>/dev/null; then
    die "daemon PID $PID still running"
  fi
else
  info "No running mynet _daemon; continuing"
fi

info "Install V0.2.9 candidate"
cp -a "$CANDIDATE" "${LIVE}.new"
chmod +x "${LIVE}.new"
go version -m "${LIVE}.new" | grep mod
mv "${LIVE}.new" "$LIVE"

info "Start with profiling"
sudo -E env "${PROF_ENV[@]}" "./$LIVE" up
sleep 15

"./$LIVE" status
"./$LIVE" diagnose --peer minipc | head -40

if ! "./$LIVE" status | grep -q "WireGuard: Active"; then
  die "WireGuard not active"
fi

info "Done. Rollback: sudo -E ./$LIVE daemon stop; cp -a $BACKUP $LIVE; sudo -E env ${PROF_ENV[*]} ./$LIVE up"
