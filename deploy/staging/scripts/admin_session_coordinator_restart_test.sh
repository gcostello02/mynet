#!/usr/bin/env bash
# Integration: coordinator restart clears in-memory admin sessions; lib-admin must recover.
# Skips when staging coordinator is not running (safe for CI without Docker).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib-compose.sh
source "${ROOT}/scripts/lib-compose.sh"
staging_compose_init "$ROOT"
# shellcheck source=lib-admin.sh
source "${ROOT}/scripts/lib-admin.sh"

if ! docker compose ps --status running coordinator 2>/dev/null | grep -q coordinator; then
  echo "admin_session_coordinator_restart_test: skip (coordinator not running)"
  exit 0
fi

staging_admin_env
if [[ ! -f "${ROOT}/admin-state/admin-session.json" ]]; then
  staging_admin_login "$ROOT" >/dev/null
fi

echo "admin_session_coordinator_restart_test: restarting coordinator (staging only)..."
docker compose restart coordinator >/dev/null
for i in $(seq 1 60); do
  if docker compose exec -T coordinator curl -sf http://127.0.0.1:8080/healthz >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

# Two ensure cycles — must not trip unbound locals from stacked RETURN traps.
staging_admin_ensure "$ROOT"
staging_admin_run "$ROOT" devices list >/dev/null
staging_admin_ensure "$ROOT"
staging_admin_run "$ROOT" invitations list >/dev/null

echo "admin_session_coordinator_restart_test: OK"
