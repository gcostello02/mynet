#!/usr/bin/env bash
# Isolated Compose smoke test for the public self-hosted coordinator image.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PROJECT="mynet-selfhosted-smoke-$$"
HOST_PORT="${MYNET_SMOKE_PORT:-$((18080 + ($(date +%s) % 2000)))}"
ENV_FILE="$(mktemp)"
BASE="http://127.0.0.1:${HOST_PORT}"

COMPOSE_ENV="${ROOT}/.env"
cleanup() {
  docker compose -f "${ROOT}/docker-compose.yml" -p "$PROJECT" down -v --remove-orphans 2>/dev/null || true
  rm -f "$ENV_FILE"
  if [[ -f "$COMPOSE_ENV" && "${COMPOSE_ENV_CREATED:-}" == 1 ]]; then
    rm -f "$COMPOSE_ENV"
  fi
}
trap cleanup EXIT

if ! command -v openssl >/dev/null; then
  echo "smoke-test: openssl required" >&2
  exit 1
fi

SECRET="$(openssl rand -base64 32)"
cat >"$ENV_FILE" <<EOF
MYNET_SERVER_SECRET=${SECRET}
MYNET_SECURE_COOKIES=false
MYNET_HOST_PORT=${HOST_PORT}
EOF

if [[ ! -f "$COMPOSE_ENV" ]]; then
  cp "$ENV_FILE" "$COMPOSE_ENV"
  COMPOSE_ENV_CREATED=1
fi

echo "smoke-test: project=${PROJECT} port=${HOST_PORT}"
docker compose -f "${ROOT}/docker-compose.yml" -p "$PROJECT" --env-file "$ENV_FILE" build --quiet
docker compose -f "${ROOT}/docker-compose.yml" -p "$PROJECT" --env-file "$ENV_FILE" up -d --wait --wait-timeout 120

if docker compose -f "${ROOT}/docker-compose.yml" -p "$PROJECT" logs coordinator 2>&1 | grep -q 'MYNET_CLIENT_NAME'; then
  echo "smoke-test: FAIL — staging client entrypoint detected in logs" >&2
  exit 1
fi

curl -sf "${BASE}/healthz" >/dev/null || { echo "smoke-test: /healthz failed" >&2; exit 1; }

STATUS="$(curl -sf "${BASE}/v1/setup/status")"
echo "$STATUS" | grep -q '"initialized":false' || { echo "smoke-test: expected uninitialized setup status, got: $STATUS" >&2; exit 1; }

HTML="$(curl -sf "${BASE}/")"
echo "$HTML" | grep -q 'MyNet Admin' && echo "$HTML" | grep -q 'id="root"' || {
  echo "smoke-test: embedded admin SPA shell not served at /" >&2
  exit 1
}
ASSET="$(echo "$HTML" | sed -n 's/.*src="\(\/assets\/index-[^"]*\.js\)".*/\1/p' | head -1)"
curl -sf "${BASE}${ASSET}" | grep -q 'Set up your coordinator' || {
  echo "smoke-test: setup wizard bundle not present" >&2
  exit 1
}

# Bootstrap capability must be available on the coordinator host (loopback inside container).
docker compose -f "${ROOT}/docker-compose.yml" -p "$PROJECT" exec -T coordinator \
  curl -sf http://127.0.0.1:8080/v1/setup/bootstrap | grep -q bootstrap_token || {
  echo "smoke-test: loopback bootstrap endpoint failed" >&2
  exit 1
}

echo "smoke-test: OK"
