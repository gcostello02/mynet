#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

COORD_URL="${MYNET_COORD_PUBLIC_URL:-https://staging-coord.mynet.test}"
ADMIN_USER="${MYNET_STAGING_ADMIN_USER:-staging-admin}"
ADMIN_PASS="${MYNET_STAGING_ADMIN_PASS:-staging-password-12chars}"

echo "Waiting for coordinator health..."
for i in $(seq 1 60); do
  if docker compose exec -T coordinator curl -sf http://127.0.0.1:8080/healthz >/dev/null; then
    break
  fi
  sleep 1
done

docker compose exec -T coordinator curl -sf -X POST http://127.0.0.1:8080/v1/admin/setup \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"${ADMIN_USER}\",\"password\":\"${ADMIN_PASS}\",\"network_display_name\":\"m2-staging\",\"public_server_url\":\"${COORD_URL}\"}" \
  | jq .

echo "Coordinator bootstrap complete."
