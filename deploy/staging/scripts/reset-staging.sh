#!/usr/bin/env bash
# Non-destructive staging reset: stop containers, clear local admin session files only.
# Coordinator SQLite volume (coord-data) is preserved.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib-compose.sh
source "${ROOT}/scripts/lib-compose.sh"
staging_compose_init "$ROOT"

if [[ "${MYNET_STAGING_RESET:-}" != "1" ]]; then
  cat <<'EOF'
Stops compose project mynet-m2-staging and removes local admin-state + invitation artifacts.
Does NOT delete the coordinator database volume.

Set MYNET_STAGING_RESET=1 to confirm.

For a new coordinator database, use: MYNET_STAGING_FRESH=1 ./scripts/fresh-staging.sh
EOF
  exit 1
fi

"${ROOT}/scripts/teardown.sh"
staging_clear_local_credentials "$ROOT"

echo "Staging stopped; coordinator DB volume preserved."
echo "Resume: docker compose up -d coordinator proxy && ./scripts/bootstrap-coordinator.sh && ./scripts/enroll-three.sh"
