#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
source "${ROOT}/scripts/lib-admin.sh"

SERVER="${MYNET_COORD_PUBLIC_URL:-https://staging-coord.mynet.test}"
ADMIN_USER="${MYNET_STAGING_ADMIN_USER:-staging-admin}"
ADMIN_PASS="${MYNET_STAGING_ADMIN_PASS:-staging-password-12chars}"

mkdir -p "${ROOT}/artifacts" "${ROOT}/admin-state"

printf '%s\n' "$ADMIN_PASS" | staging_admin "$ROOT" login --server "$SERVER" --user "$ADMIN_USER" || true

for n in client-a client-b client-c; do
  echo "Creating invitation for ${n}..."
  out="$(staging_admin "$ROOT" invitations create --label "$n" --expires 86400 --max-uses 1)"
  inv="$(echo "$out" | tail -1)"
  printf '%s' "$inv" >"${ROOT}/artifacts/inv-${n}.txt"
  echo "Wrote artifacts/inv-${n}.txt"
done

staging_admin "$ROOT" devices list | tee "${ROOT}/artifacts/devices-after-enroll.json"
