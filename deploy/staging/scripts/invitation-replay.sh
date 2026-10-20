#!/usr/bin/env bash
# Replay a consumed invitation from an isolated join context (no daemon, no prior identity).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib-compose.sh
source "${ROOT}/scripts/lib-compose.sh"
staging_compose_init "$ROOT"
# shellcheck source=lib-admin.sh
source "${ROOT}/scripts/lib-admin.sh"
# shellcheck source=lib-invitation-replay.sh
source "${ROOT}/scripts/lib-invitation-replay.sh"

ART="${ROOT}/artifacts"
INV_FILE="${ART}/inv-client-c.txt"
LOG="${ART}/invitation-replay-results.txt"
mkdir -p "$ART"
: >"$LOG"

log() { echo "$*" | tee -a "$LOG"; }
fail() { log "FAIL: $*"; exit 1; }

[[ -s "$INV_FILE" ]] || staging_die "missing ${INV_FILE} (run enroll-three first)"

staging_admin_ensure "$ROOT"

devices_before="$(staging_admin_devices_json "$ROOT" | jq '[.[] | select(.revoked == false)] | length')"
uses_before="$(staging_admin_run "$ROOT" invitations list | jq '[.[] | select(.label == "client-c") | .uses] | first // -1')"
log "baseline: active_devices=${devices_before} client-c_invitation_uses=${uses_before}"

log "Isolated join replay (invitation-replay service, tmpfs state, no daemon)..."
replay_out="$(mktemp)"
replay_err="$(mktemp)"
set +e
(
  cd "$ROOT"
  export COMPOSE_PROJECT_NAME=mynet-m2-staging
  docker compose --profile tools run --rm --no-deps --quiet-pull invitation-replay \
    -lc 'update-ca-certificates >/dev/null 2>&1
test -d /replay-state && test ! -f /replay-state/identity.json
inv="$(tr -d "\n" </artifacts/inv-client-c.txt)"
exec mynet join --name replay-probe "$inv"'
) >"$replay_out" 2>"$replay_err"
replay_ec=$?
set -e

if [[ -s "$replay_err" ]]; then
  log "compose diagnostics (stderr, bounded):"
  head -8 "$replay_err" | while IFS= read -r line; do log "  $line"; done
fi

safe_lines="$(invitation_replay_sanitize_log "$(cat "$replay_out")")"
log "join exit_code=${replay_ec}"
while IFS= read -r line; do
  [[ -n "$line" ]] && log "  $line"
done <<<"$safe_lines"

kind="$(classify_replay_join_result "$(cat "$replay_out")$(cat "$replay_err")" "$replay_ec")"
log "classification: ${kind}"

rm -f "$replay_out" "$replay_err"

case "$kind" in
  success)
    fail "join succeeded; expected coordinator rejection of consumed invitation"
    ;;
  coordinator_rejected)
    log "coordinator rejected replay (invitation exhausted or enrollment denied)"
    ;;
  docker_compose_error)
    fail "docker compose invocation error (check service invitation-replay and flag order)"
    ;;
  local_validation)
    fail "join failed local validation before coordinator enrollment"
    ;;
  tls_connectivity)
    fail "TLS or network error reaching staging coordinator"
    ;;
  auth_denied)
    fail "enrollment rejected for auth/signature (not the same as consumed invitation replay)"
    ;;
  *)
    fail "join failed without coordinator replay rejection (classification=${kind})"
    ;;
esac

devices_after="$(staging_admin_devices_json "$ROOT" | jq '[.[] | select(.revoked == false)] | length')"
uses_after="$(staging_admin_run "$ROOT" invitations list | jq '[.[] | select(.label == "client-c") | .uses] | first // -1')"
if [[ "$devices_after" != "$devices_before" ]]; then
  fail "active device count changed ${devices_before} -> ${devices_after}"
fi
if [[ "$uses_after" != "$uses_before" ]]; then
  fail "invitation uses changed ${uses_before} -> ${uses_after} (replay must not consume)"
fi
if staging_admin_devices_json "$ROOT" | jq -e '.[] | select(.name == "replay-probe")' >/dev/null 2>&1; then
  fail "replay-probe device was created"
fi

log "PASS: coordinator rejected consumed invitation; counts unchanged"
log "Results: $LOG"
