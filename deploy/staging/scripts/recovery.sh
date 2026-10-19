#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
ART="${ROOT}/artifacts/recovery-results.txt"
: >"$ART"

log() { echo "$*" | tee -a "$ART"; }

log "Restart client-b container (simulate crash)..."
docker compose restart client-b
sleep 15
docker compose exec -T client-a ping -c 2 "$(docker compose exec -T client-b jq -r .vpn_ipv4 /var/lib/mynet/wg.json)" | tee -a "$ART" || log "WARN ping after client-b restart"

log "Restart coordinator (staging only)..."
docker compose restart coordinator
sleep 5
docker compose exec -T coordinator curl -sf http://127.0.0.1:8080/healthz | tee -a "$ART"

log "Check client-a still has identity..."
docker compose exec -T client-a test -f /var/lib/mynet/identity.json && log "identity ok"

log "Recovery script done"
