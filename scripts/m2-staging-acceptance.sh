#!/usr/bin/env bash
# Isolated M2 staging (non-production). Requires: go, openssl or mkcert, three empty state dirs.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
STAGE_ROOT="${STAGE_ROOT:-/tmp/mynet-m2-staging}"
COORD_DB="${STAGE_ROOT}/coordinator.db"
COORD_PORT="${COORD_PORT:-18787}"
SECRET_FILE="${STAGE_ROOT}/server.secret"

mkdir -p "$STAGE_ROOT"
if [[ ! -f "$SECRET_FILE" ]]; then
  openssl rand -base64 32 >"$SECRET_FILE"
fi
export MYNET_SERVER_SECRET="$(tr -d '\n' <"$SECRET_FILE")"
export MYNET_DATA_PATH="$COORD_DB"

echo "Build binaries..."
go build -o "$STAGE_ROOT/mynet-server" "$ROOT/cmd/mynet-server"
go build -o "$STAGE_ROOT/mynet" "$ROOT/cmd/mynet"

echo "Start coordinator on 127.0.0.1:${COORD_PORT} (HTTP for bootstrap; use reverse TLS proxy for HTTPS join in production-like staging)."
echo "Complete admin setup via localhost POST /v1/admin/setup, then admin login + invitations + three joins with separate MYNET_STATE_DIR."
echo "Do NOT point at mesh.example.com or reuse production credentials."
