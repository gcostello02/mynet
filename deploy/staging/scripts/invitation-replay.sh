#!/usr/bin/env bash
# Replay a consumed invitation from an isolated join context (no daemon, no prior identity).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib-compose.sh
source "${ROOT}/scripts/lib-compose.sh"
staging_compose_init "$ROOT"
# shellcheck source=lib-admin.sh
source "${ROOT}/scripts/lib-admin.sh"

ART="${ROOT}/artifacts"
INV_FILE="${ART}/inv-client-c.txt"
LOG="${ART}/invitation-replay-results.txt"
mkdir -p "$ART"
: >"$LOG"

log() { echo "$*" | tee -a "$LOG"; }

[[ -s "$INV_FILE" ]] || staging_die "missing ${INV_FILE} (run enroll-three first)"

staging_admin_ensure "$ROOT"

devices_before="$(staging_admin_run "$ROOT" devices list | jq '[.[] | select(.revoked == false)] | length')"
uses_before="$(staging_admin_run "$ROOT" invitations list | jq '[.[] | select(.label == "client-c") | .uses] | first // -1')"
log "baseline: active_devices=${devices_before} client-c_invitation_uses=${uses_before}"

log "Isolated join replay (empty state dir, no mynet daemon)..."
set +e
replay_out="$(
  docker compose run --rm --no-deps \
    -v "${ART}:/artifacts:ro" \
    --tmpfs /tmp/replay-join:rw,mode=0700 \
    -e MYNET_STATE_DIR=/tmp/replay-join \
    admin-toolbox -lc '
      update-ca-certificates >/dev/null 2>&1
      inv="$(tr -d "\n" </artifacts/inv-client-c.txt)"
      mynet join --name replay-probe "$inv"
    ' 2>&1
)"
replay_ec=$?
set -e

# Never log invitation material; only status lines and coordinator errors.
safe_lines="$(echo "$replay_out" | grep -vE '^[A-Za-z0-9_-]{40,}$' | head -20)"
log "join exit_code=${replay_ec}"
while IFS= read -r line; do
  [[ -n "$line" ]] && log "  $line"
done <<<"$safe_lines"

if [[ "$replay_ec" -eq 0 ]]; then
  log "FAIL: join succeeded; expected coordinator rejection of consumed invitation"
  exit 1
fi
if ! echo "$replay_out" | grep -qiE 'http 410|exhausted|enrollment denied'; then
  log "FAIL: join failed locally without coordinator rejection (no http 410 / exhausted)"
  exit 1
fi

devices_after="$(staging_admin_run "$ROOT" devices list | jq '[.[] | select(.revoked == false)] | length')"
uses_after="$(staging_admin_run "$ROOT" invitations list | jq '[.[] | select(.label == "client-c") | .uses] | first // -1')"
if [[ "$devices_after" != "$devices_before" ]]; then
  log "FAIL: active device count changed ${devices_before} -> ${devices_after}"
  exit 1
fi
if [[ "$uses_after" != "$uses_before" ]]; then
  log "FAIL: invitation uses changed ${uses_before} -> ${uses_after} (replay must not consume)"
  exit 1
fi
if staging_admin_run "$ROOT" devices list | jq -e '.[] | select(.name == "replay-probe")' >/dev/null 2>&1; then
  log "FAIL: replay-probe device was created"
  exit 1
fi

log "PASS: coordinator rejected consumed invitation; counts unchanged"
log "Results: $LOG"
