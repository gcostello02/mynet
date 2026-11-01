#!/usr/bin/env bash
# Tear down staging stack (safe if production is not using this compose project).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib-compose.sh
source "${ROOT}/scripts/lib-compose.sh"
staging_compose_init "$ROOT"

docker compose --profile clients down --remove-orphans 2>/dev/null || true
docker compose --profile tools down --remove-orphans 2>/dev/null || true
docker compose down --remove-orphans 2>/dev/null || true

if [[ "${MYNET_STAGING_PURGE_VOLUMES:-0}" == "1" ]]; then
  docker compose --profile clients down -v --remove-orphans 2>/dev/null || true
  docker compose down -v --remove-orphans 2>/dev/null || true
fi

if [[ "${MYNET_STAGING_PURGE_ARTIFACTS:-0}" == "1" ]]; then
  rm -rf "${ROOT}/artifacts/"* "${ROOT}/certs/"*.crt "${ROOT}/certs/"*.key "${ROOT}/certs/"*.csr "${ROOT}/certs/"*.srl 2>/dev/null || true
fi

echo "Staging compose project stopped (project=${COMPOSE_PROJECT_NAME}). Production mesh unchanged."
echo "To delete staging volumes: MYNET_STAGING_PURGE_VOLUMES=1 $0"
