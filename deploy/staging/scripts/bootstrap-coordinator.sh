#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib-compose.sh
source "${ROOT}/scripts/lib-compose.sh"
staging_compose_init "$ROOT"

COORD_URL="${MYNET_COORD_PUBLIC_URL:-https://staging-coord.mynet.test}"
ADMIN_USER="${MYNET_STAGING_ADMIN_USER:-staging-admin}"
ADMIN_PASS="${MYNET_STAGING_ADMIN_PASS:-staging-password-12chars}"

echo "Waiting for coordinator health..."
for i in $(seq 1 60); do
  if docker compose exec -T coordinator curl -sf http://127.0.0.1:8080/healthz >/dev/null 2>&1; then
    break
  fi
  if [[ "$i" -eq 60 ]]; then
    staging_die "coordinator not healthy after 60s"
  fi
  sleep 1
done

if docker compose exec -T coordinator curl -sf http://127.0.0.1:8080/v1/network/public >/dev/null 2>&1; then
  echo "Coordinator already bootstrapped (network public available); skipping setup."
  exit 0
fi

tmp="$(mktemp)"
code="$(docker compose exec -T coordinator curl -s -o "$tmp" -w '%{http_code}' -X POST http://127.0.0.1:8080/v1/admin/setup \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"${ADMIN_USER}\",\"password\":\"${ADMIN_PASS}\",\"network_display_name\":\"m2-staging\",\"public_server_url\":\"${COORD_URL}\"}")"
body="$(cat "$tmp")"
rm -f "$tmp"

case "$code" in
  201)
    echo "$body" | jq -r '.admin_id,.coordinator_fingerprint // empty' 2>/dev/null | while read -r line; do
      [[ -n "$line" ]] && echo "bootstrap: $line"
    done
    echo "Coordinator bootstrap complete."
    ;;
  403)
    if echo "$body" | grep -qi 'already'; then
      echo "Coordinator already bootstrapped; resuming."
      exit 0
    fi
    staging_die "setup forbidden (http $code): $body"
    ;;
  *)
    staging_die "setup failed (http $code): $body"
    ;;
esac
