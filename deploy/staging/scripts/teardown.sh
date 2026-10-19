#!/usr/bin/env bash
# Tear down staging stack (safe if production is not using this compose project).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

export COMPOSE_PROJECT_NAME=mynet-m2-staging

docker compose --profile clients down -v --remove-orphans 2>/dev/null || true
docker compose down -v --remove-orphans 2>/dev/null || true

if [[ "${MYNET_STAGING_PURGE_ARTIFACTS:-0}" == "1" ]]; then
  rm -rf "${ROOT}/artifacts/"* "${ROOT}/certs/"*.crt "${ROOT}/certs/"*.key "${ROOT}/certs/"*.csr "${ROOT}/certs/"*.srl 2>/dev/null || true
fi

echo "Staging compose project removed. Production mesh unchanged."
