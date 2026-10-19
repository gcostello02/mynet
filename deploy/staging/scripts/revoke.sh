#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
# shellcheck source=lib-admin.sh
source "${ROOT}/scripts/lib-admin.sh"
ART="${ROOT}/artifacts"
mkdir -p "$ART"
LOG="${ART}/revoke-results.txt"
: >"$LOG"

log() { echo "$*" | tee -a "$LOG"; }
fail() { log "FAIL: $*"; exit 1; }

log "Ensuring admin session before revocation measurements..."
staging_admin_ensure "$ROOT" || fail "admin authentication"

DEVICE_ID="$(docker compose exec -T client-c sh -c 'jq -r .device_id /var/lib/mynet/identity.json')"
VPN_C="$(docker compose exec -T client-c jq -r .vpn_ipv4 /var/lib/mynet/wg.json)"
VPN_A="$(docker compose exec -T client-a jq -r .vpn_ipv4 /var/lib/mynet/wg.json)"
VPN_B="$(docker compose exec -T client-b jq -r .vpn_ipv4 /var/lib/mynet/wg.json)"

if staging_admin_run "$ROOT" devices list | jq -e --arg id "$DEVICE_ID" '.[] | select(.device_id == $id and .revoked == true)' >/dev/null 2>&1; then
  fail "device already revoked before test"
fi

log "Revoking device_id=${DEVICE_ID} (client-c) at $(date -u +%Y-%m-%dT%H:%M:%SZ)"
T_REVOKE=$(date +%s.%N)
staging_admin_run "$ROOT" devices revoke "$DEVICE_ID" | tee -a "$LOG"

T_COORD_REVOKED=""
for i in $(seq 1 30); do
  if staging_admin_run "$ROOT" devices list | jq -e --arg id "$DEVICE_ID" '.[] | select(.device_id == $id and .revoked == true)' >/dev/null; then
    T_COORD_REVOKED=$(date +%s.%N)
    log "Coordinator reports revoked (poll ${i})"
    break
  fi
  sleep 0.5
done
[[ -n "$T_COORD_REVOKED" ]] || fail "coordinator never reported device revoked"

log "Revoked device auth should fail (device session refresh)..."
set +e
auth_out="$(docker compose exec -T client-c mynet login 2>&1)"
auth_ec=$?
set -e
echo "$auth_out" | grep -vE 'Bearer|token|session' | head -10 | tee -a "$LOG" || true
if [[ "$auth_ec" -eq 0 ]] && ! echo "$auth_out" | grep -qiE 'revoked|denied|http 401|http 403'; then
  fail "revoked device auth unexpectedly succeeded"
fi
log "Revoked device auth rejected (exit ${auth_ec})"

T_PING_FAIL=""
for i in $(seq 1 120); do
  if ! docker compose exec -T client-a ping -c 1 -W 1 "$VPN_C" >/dev/null 2>&1; then
    T_PING_FAIL=$(date +%s.%N)
    log "VPN ping A->C blocked (iteration $i)"
    break
  fi
  sleep 1
done
[[ -n "$T_PING_FAIL" ]] || fail "client-a still pings revoked VPN IP after 120s"

T_WG_REMOVED=""
for i in $(seq 1 120); do
  if ! docker compose exec -T client-a wg show 2>/dev/null | grep -qF "${VPN_C}/32"; then
    T_WG_REMOVED=$(date +%s.%N)
    log "WireGuard peer for client-c removed on client-a (iteration $i)"
    break
  fi
  sleep 1
done
[[ -n "$T_WG_REMOVED" ]] || fail "client-a wg still lists revoked peer after 120s"

T_ICE_CLEAR=""
for i in $(seq 1 120); do
  diag="$(docker compose exec -T client-a mynet diagnose --peer client-c 2>&1 || true)"
  if echo "$diag" | grep -qiE 'not found|no peer|disconnected|idle|awaiting|n/a'; then
    if ! echo "$diag" | grep -qiE 'State:\s+connected|ICE State:\s+connected'; then
      T_ICE_CLEAR=$(date +%s.%N)
      log "ICE session for client-c cleared on client-a (iteration $i)"
      break
    fi
  fi
  if ! echo "$diag" | grep -q "$VPN_C"; then
    T_ICE_CLEAR=$(date +%s.%N)
    log "diagnose no longer references revoked VPN IP (iteration $i)"
    break
  fi
  sleep 1
done
[[ -n "$T_ICE_CLEAR" ]] || log "WARN: could not confirm ICE teardown via diagnose (continuing)"

log "Remaining mesh: client-a -> client-b"
docker compose exec -T client-a ping -c 2 "$VPN_B" | tee -a "$LOG"
docker compose exec -T client-a mynet ping --peer client-b 2>&1 | tee -a "$LOG"

fmt_delay() {
  awk -v a="$1" -v b="$2" 'BEGIN { d = b - a; if (d < 0) d = 0; printf "%.3f", d }'
}

log "--- enforcement latency (seconds from revoke API) ---"
log "coordinator_revoked: $(fmt_delay "$T_REVOKE" "$T_COORD_REVOKED")s"
log "vpn_ping_blocked:    $(fmt_delay "$T_REVOKE" "$T_PING_FAIL")s"
log "wg_peer_removed:     $(fmt_delay "$T_REVOKE" "$T_WG_REMOVED")s"
if [[ -n "$T_ICE_CLEAR" ]]; then
  log "ice_session_cleared: $(fmt_delay "$T_REVOKE" "$T_ICE_CLEAR")s"
fi

log "Consumed-invitation replay (coordinator must reject):"
"${ROOT}/scripts/invitation-replay.sh" | tee -a "$LOG"

log "Revocation acceptance complete (see $LOG)"
