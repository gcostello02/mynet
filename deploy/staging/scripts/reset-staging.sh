#!/usr/bin/env bash
# Reset ONLY the mynet-m2-staging compose project (volumes + local admin state).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib-compose.sh
source "${ROOT}/scripts/lib-compose.sh"
staging_compose_init "$ROOT"

if [[ "${MYNET_STAGING_RESET:-}" != "1" ]]; then
  cat <<'EOF'
This removes Docker resources for compose project mynet-m2-staging only.
Set MYNET_STAGING_RESET=1 to confirm.

Does not touch other compose projects or production MyNet.
EOF
  exit 1
fi

"${ROOT}/scripts/teardown.sh"
rm -rf "${ROOT}/admin-state"
if [[ "${MYNET_STAGING_PURGE_SECRETS:-0}" == "1" ]]; then
  rm -f "${ROOT}/secrets/server.secret"
fi
echo "Staging reset complete. Re-run acceptance from a clean coordinator DB."
