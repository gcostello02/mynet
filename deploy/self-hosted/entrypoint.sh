#!/usr/bin/env bash
# Self-hosted coordinator only — runs mynet-server (not the staging VPN client entrypoint).
set -euo pipefail

if [[ -z "${MYNET_SERVER_SECRET:-}" ]]; then
  echo "mynet-coordinator: MYNET_SERVER_SECRET is required (at least 32 bytes). See deploy/coordinator/.env.example" >&2
  exit 1
fi

DATA="${MYNET_DATA_PATH:-/data/coordinator.db}"
LISTEN="${MYNET_LISTEN:-:8080}"

exec /usr/local/bin/mynet-server --listen "$LISTEN" --data "$DATA"
