#!/usr/bin/env bash
# Safe smoke test: bash, CA trust, HTTPS to staging proxy (no privileged clients).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib-compose.sh
source "${ROOT}/scripts/lib-compose.sh"
staging_compose_init "$ROOT"
staging_load_server_secret "$ROOT"

docker compose up -d coordinator proxy
"${ROOT}/scripts/bootstrap-coordinator.sh"

echo "Checking admin-toolbox bash..."
docker compose run --rm --no-deps admin-toolbox -lc 'echo bash-ok'

echo "Checking HTTPS via staging-coord.mynet.test..."
docker compose run --rm --no-deps admin-toolbox -lc \
  'update-ca-certificates >/dev/null 2>&1; curl -sf https://staging-coord.mynet.test/healthz'

echo "Checking admin login (password not logged)..."
ADMIN_USER="${MYNET_STAGING_ADMIN_USER:-staging-admin}"
ADMIN_PASS="${MYNET_STAGING_ADMIN_PASS:-staging-password-12chars}"
rm -rf "${ROOT}/admin-state"
mkdir -p "${ROOT}/admin-state"
# shellcheck source=lib-admin.sh
source "${ROOT}/scripts/lib-admin.sh"
printf '%s\n' "$ADMIN_PASS" | staging_admin "$ROOT" login --server https://staging-coord.mynet.test --user "$ADMIN_USER"

echo "Checking invitation create (token not printed)..."
out="$(staging_admin "$ROOT" invitations create --label verify-smoke --expires 3600 --max-uses 1)"
inv="$(echo "$out" | tail -1)"
if [[ ${#inv} -lt 20 ]]; then
  staging_die "invitation create failed"
fi
echo "verify-admin-toolbox: OK"
