#!/usr/bin/env bash
# ICMP + mynet ping between staging clients. Requires privileged client containers up.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
ART="${ROOT}/artifacts"
mkdir -p "$ART"
RESULT="${ART}/data-plane-results.txt"
EVIDENCE="${ART}/data-plane-evidence.txt"
: >"$RESULT"
: >"$EVIDENCE"

log() { echo "$*" | tee -a "$RESULT"; }
ev() { echo "$*" >>"$EVIDENCE"; }

FAILURES=0
record_fail() {
  log "FAIL: $*"
  FAILURES=$((FAILURES + 1))
}

wait_vpn_active() {
  local c="$1"
  for i in $(seq 1 90); do
    if docker compose exec -T "$c" mynet status 2>/dev/null | grep -q 'WireGuard:.*Active'; then
      return 0
    fi
    sleep 2
  done
  return 1
}

wait_icmp() {
  local from="$1" ip="$2" label="$3"
  for i in $(seq 1 60); do
    if docker compose exec -T "$from" ping -c 1 -W 2 "$ip" >/dev/null 2>&1; then
      log "ICMP ready ${label} (attempt $i)"
      return 0
    fi
    sleep 2
  done
  return 1
}

collect_evidence() {
  ev "=== evidence $(date -u +%Y-%m-%dT%H:%M:%SZ) ==="
  for c in client-a client-b client-c; do
    ev "--- mynet status ${c} ---"
    docker compose exec -T "$c" mynet status 2>&1 | head -30 >>"$EVIDENCE" || ev "(status failed)"
    ev "--- wg show ${c} ---"
    docker compose exec -T "$c" wg show 2>&1 | head -40 >>"$EVIDENCE" || ev "(wg show failed)"
    for peer in client-a client-b client-c; do
      [[ "$peer" == "$c" ]] && continue
      ev "--- diagnose ${c} -> ${peer} ---"
      docker compose exec -T "$c" mynet diagnose --peer "$peer" 2>&1 | head -35 >>"$EVIDENCE" || true
    done
  done
}

for c in client-a client-b client-c; do
  if [[ -f "${ART}/${c}.failed" ]]; then
    log "client ${c} failed: $(cat "${ART}/${c}.failed")"
    exit 1
  fi
  if [[ ! -f "${ART}/${c}.vpn-ready" ]]; then
    record_fail "${c} missing vpn-ready marker"
    continue
  fi
  log "Waiting for WireGuard active on $c..."
  if ! wait_vpn_active "$c"; then
    record_fail "$c WireGuard not active within timeout"
  fi
done

ip_a="$(docker compose exec -T client-a sh -c 'jq -r .vpn_ipv4 /var/lib/mynet/wg.json 2>/dev/null')"
ip_b="$(docker compose exec -T client-b sh -c 'jq -r .vpn_ipv4 /var/lib/mynet/wg.json 2>/dev/null')"
ip_c="$(docker compose exec -T client-c sh -c 'jq -r .vpn_ipv4 /var/lib/mynet/wg.json 2>/dev/null')"
log "VPN IPs: a=$ip_a b=$ip_b c=$ip_c"

for pair in "client-a:$ip_b:a->b" "client-a:$ip_c:a->c" "client-b:$ip_c:b->c"; do
  IFS=: read -r from ip label <<<"$pair"
  if ! wait_icmp "$from" "$ip" "$label"; then
    record_fail "ICMP $label not ready"
  fi
done

for pair in "client-a:$ip_b:client-b" "client-a:$ip_c:client-c" "client-b:$ip_c:client-c"; do
  IFS=: read -r from ip _ <<<"$pair"
  log "ICMP $from -> $ip"
  if ! docker compose exec -T "$from" ping -c 3 -W 2 "$ip" | tee -a "$RESULT"; then
    record_fail "ICMP $from->$ip"
  fi
done

for pair in "client-a:client-b" "client-a:client-c" "client-b:client-c"; do
  IFS=: read -r from peer <<<"$pair"
  log "mynet ping $from -> $peer"
  if ! docker compose exec -T "$from" mynet ping --peer "$peer" 2>&1 | tee -a "$RESULT"; then
    record_fail "mynet ping $from->$peer"
  fi
done

collect_evidence

if [[ "$FAILURES" -gt 0 ]]; then
  log "Data plane failed ($FAILURES checks); evidence: $EVIDENCE"
  exit 1
fi

log "Data plane passed (see $RESULT)"
log "Evidence: $EVIDENCE"
