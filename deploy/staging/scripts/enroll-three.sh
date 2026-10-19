#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib-compose.sh
source "${ROOT}/scripts/lib-compose.sh"
staging_compose_init "$ROOT"
source "${ROOT}/scripts/lib-admin.sh"

SERVER="${MYNET_COORD_PUBLIC_URL:-https://staging-coord.mynet.test}"
ADMIN_USER="${MYNET_STAGING_ADMIN_USER:-staging-admin}"
ADMIN_PASS="${MYNET_STAGING_ADMIN_PASS:-staging-password-12chars}"

mkdir -p "${ROOT}/artifacts" "${ROOT}/admin-state"

staging_admin_env
export MYNET_COORD_PUBLIC_URL="$SERVER"
export MYNET_STAGING_ADMIN_USER="$ADMIN_USER"
export MYNET_STAGING_ADMIN_PASS="$ADMIN_PASS"
if [[ ! -f "${ROOT}/admin-state/admin-session.json" ]]; then
  staging_admin_login "$ROOT"
else
  staging_admin_ensure "$ROOT" || staging_admin_login "$ROOT"
fi

for n in client-a client-b client-c; do
  if [[ -s "${ROOT}/artifacts/inv-${n}.txt" ]]; then
    echo "Invitation inv-${n}.txt already exists; skipping create."
    continue
  fi
  echo "Creating invitation for ${n}..."
  out="$(staging_admin_run "$ROOT" invitations create --label "$n" --expires 86400 --max-uses 1)"
  inv="$(echo "$out" | tail -1)"
  if [[ ${#inv} -lt 20 ]]; then
    staging_die "invitation output looks invalid for ${n} (check admin login / TLS)"
  fi
  printf '%s' "$inv" >"${ROOT}/artifacts/inv-${n}.txt"
  echo "Wrote artifacts/inv-${n}.txt (token not printed)."
done

staging_admin_run "$ROOT" devices list | tee "${ROOT}/artifacts/devices-after-enroll.json"
