#!/usr/bin/env bash
# ICMP + mynet ping between staging clients. Requires privileged client containers up.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
ART="${ROOT}/artifacts"
mkdir -p "$ART"
RESULT="${ART}/data-plane-results.txt"
: >"$RESULT"

log() { echo "$*" | tee -a "$RESULT"; }

wait_vpn() {
  local c="$1"
  for i in $(seq 1 90); do
    if docker compose exec -T "$c" mynet status 2>/dev/null | grep -qi 'vpn'; then
      return 0
    fi
    sleep 2
  done
  return 1
}

for c in client-a client-b client-c; do
  if [[ -f "${ART}/${c}.failed" ]]; then
    log "client ${c} failed: $(cat "${ART}/${c}.failed")"
    exit 1
  fi
  if [[ ! -f "${ART}/${c}.vpn-ready" ]]; then
    log "WARN: ${c} missing vpn-ready marker"
  fi
  log "Waiting for VPN on $c..."
  wait_vpn "$c" || log "WARN: $c VPN status not confirmed"
done

ip_a="$(docker compose exec -T client-a sh -c 'cat /var/lib/mynet/wg.json 2>/dev/null | jq -r .vpn_ipv4')"
ip_b="$(docker compose exec -T client-b sh -c 'cat /var/lib/mynet/wg.json 2>/dev/null | jq -r .vpn_ipv4')"
ip_c="$(docker compose exec -T client-c sh -c 'cat /var/lib/mynet/wg.json 2>/dev/null | jq -r .vpn_ipv4')"
log "VPN IPs: a=$ip_a b=$ip_b c=$ip_c"

for pair in "client-a:$ip_b:client-b" "client-a:$ip_c:client-c" "client-b:$ip_c:client-c"; do
  IFS=: read -r from ip _ <<<"$pair"
  log "ICMP $from -> $ip"
  docker compose exec -T "$from" ping -c 3 -W 2 "$ip" | tee -a "$RESULT" || log "FAIL icmp $from->$ip"
done

for pair in "client-a:client-b" "client-a:client-c" "client-b:client-c"; do
  IFS=: read -r from peer <<<"$pair"
  log "mynet ping $from -> $peer"
  docker compose exec -T "$from" mynet ping --peer "$peer" 2>&1 | tee -a "$RESULT" || log "FAIL mynet ping $from->$peer"
done

for c in client-a client-b client-c; do
  log "diagnose $c"
  docker compose exec -T "$c" mynet diagnose 2>&1 | head -40 | tee -a "$RESULT" || true
done

log "Data plane script finished (see $RESULT)"
