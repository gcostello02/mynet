#!/usr/bin/env bash
# MyNet V0.2.9 controlled rollout — Ubuntu mini PC (client daemon only).
# Run interactively on the mini PC: bash scripts/deploy-v029-mini.sh
# Requires passworded sudo when prompted. Does not touch coordinator/tunnel/Tailscale/SSH.
set -euo pipefail

REPO="${MYNET_REPO:-$HOME/mynet}"
EXPECTED_COMMIT="21092d3aa8c54b7b5a079c62fb88d78f934adc0f"
CANDIDATE="bin/mynet-v029-candidate"
BACKUP="bin/mynet.v028-good"
LIVE="bin/mynet"
PROF_ENV=(MYNET_ENABLE_PPROF=1)

die() { echo "ERROR: $*" >&2; exit 1; }
info() { echo "==> $*"; }

info "Host check (expect Linux amd64 mini PC)"
[[ "$(uname -s)" == "Linux" ]] || die "not Linux"
[[ "$(uname -m)" == "x86_64" ]] || die "not amd64"
HOST="$(hostname -s 2>/dev/null || hostname)"
[[ "$HOST" == "minipc" ]] || echo "WARN: hostname is '$HOST' (expected minipc); continuing if repo path is correct"

cd "$REPO" || die "repo missing: $REPO"

ACTUAL="$(git rev-parse HEAD)"
[[ "$ACTUAL" == "$EXPECTED_COMMIT" ]] || die "git HEAD $ACTUAL != $EXPECTED_COMMIT"

[[ -x "$BACKUP" ]] || die "missing V0.2.8 backup $BACKUP (cp -a $LIVE $BACKUP)"
[[ -x "$CANDIDATE" ]] || die "missing candidate $CANDIDATE (go build -o $CANDIDATE ./cmd/mynet)"

info "Verify backup is V0.2.8-era binary"
if ! go version -m "$BACKUP" 2>/dev/null | grep -q 'f243542'; then
  die "backup does not look like f243542 build; refresh: cp -a $LIVE $BACKUP while still on V0.2.8"
fi

if ! go version -m "$CANDIDATE" 2>/dev/null | grep -q '21092d3'; then
  die "candidate is not 21092d3; rebuild: go build -o $CANDIDATE ./cmd/mynet"
fi

info "Identify MyNet client daemon (exact path, no broad pkill)"
mapfile -t PIDS < <(pgrep -f "${REPO}/${LIVE} _daemon" || true)
if [[ ${#PIDS[@]} -eq 0 ]]; then
  mapfile -t PIDS < <(pgrep -f "mynet _daemon" || true)
fi
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
go version -m "${LIVE}.new" | grep mod || die "cannot read candidate build info"
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
