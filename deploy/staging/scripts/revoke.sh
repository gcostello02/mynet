#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
source "${ROOT}/scripts/lib-admin.sh"
ART="${ROOT}/artifacts"
mkdir -p "$ART"
LOG="${ART}/revoke-results.txt"
: >"$LOG"

log() { echo "$*" | tee -a "$LOG"; }

DEVICE_ID="$(docker compose exec -T client-c sh -c 'jq -r .device_id /var/lib/mynet/identity.json')"
VPN_C="$(docker compose exec -T client-c jq -r .vpn_ipv4 /var/lib/mynet/wg.json)"
VPN_A="$(docker compose exec -T client-a jq -r .vpn_ipv4 /var/lib/mynet/wg.json)"

log "Revoking device_id=$DEVICE_ID (client-c) at $(date -u +%Y-%m-%dT%H:%M:%SZ)"
START=$(date +%s)
staging_admin "$ROOT" devices revoke "$DEVICE_ID" | tee -a "$LOG"

log "Polling until client-a cannot ping $VPN_C ..."
for i in $(seq 1 120); do
  if ! docker compose exec -T client-a ping -c 1 -W 1 "$VPN_C" >/dev/null 2>&1; then
    END=$(date +%s)
    log "Revocation dataplane delay: $((END-START))s (iteration $i)"
    break
  fi
  sleep 1
done

log "Auth for revoked device should fail:"
docker compose exec -T client-c mynet login 2>&1 | tee -a "$LOG" || true

log "Re-join with old invitation should fail:"
if [[ -f "${ART}/inv-client-c.txt" ]]; then
  docker compose exec -T client-c sh -c "rm -f /var/lib/mynet/identity.json /var/lib/mynet/session.json" || true
  inv="$(tr -d '\n' <"${ART}/inv-client-c.txt")"
  docker compose exec -T client-c mynet join --name client-c "$inv" 2>&1 | tee -a "$LOG" || log "join rejected as expected"
fi

log "client-a -> client-b still up:"
docker compose exec -T client-a ping -c 2 "$(docker compose exec -T client-b jq -r .vpn_ipv4 /var/lib/mynet/wg.json)" | tee -a "$LOG"
