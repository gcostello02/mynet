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

# Body must be read from curl stdout inside the container (host mktemp paths are not visible in exec).
raw="$(docker compose exec -T coordinator curl -s -w $'\n__HTTP_STATUS__%{http_code}\n' -X POST http://127.0.0.1:8080/v1/admin/setup \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"${ADMIN_USER}\",\"password\":\"${ADMIN_PASS}\",\"network_display_name\":\"m2-staging\",\"public_server_url\":\"${COORD_URL}\"}")"
code="$(echo "$raw" | tail -1 | sed 's/__HTTP_STATUS__//')"
body="$(echo "$raw" | sed '$d')"

case "$code" in
  201)
    if command -v jq >/dev/null 2>&1 && echo "$body" | jq -e . >/dev/null 2>&1; then
      aid="$(echo "$body" | jq -r '.admin_id // empty')"
      [[ -n "$aid" ]] && echo "bootstrap: admin_id=${aid}"
    fi
    echo "Coordinator bootstrap complete."
    ;;
  403)
    if echo "$body" | grep -qi 'already'; then
      echo "Coordinator already bootstrapped; resuming."
      exit 0
    fi
    staging_die "setup forbidden (http $code)"
    ;;
  000|"")
    staging_die "setup request failed (no http response; is coordinator running?)"
    ;;
  *)
    staging_die "setup failed (http $code)"
    ;;
esac
