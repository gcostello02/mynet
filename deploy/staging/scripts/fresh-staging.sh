#!/usr/bin/env bash
# Destructive fresh staging: removes ONLY mynet-m2-staging compose volumes + local secrets.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib-compose.sh
source "${ROOT}/scripts/lib-compose.sh"
staging_compose_init "$ROOT"

if [[ "${MYNET_STAGING_FRESH:-}" != "1" ]]; then
  cat <<'EOF'
Deletes coordinator SQLite and all compose volumes for project mynet-m2-staging only.
Also removes secrets/server.secret and local admin-state (new bootstrap required).

Set MYNET_STAGING_FRESH=1 to confirm.

Does not remove unrelated Docker volumes or production MyNet.
EOF
  exit 1
fi

export MYNET_STAGING_PURGE_VOLUMES=1
"${ROOT}/scripts/teardown.sh"
staging_clear_local_credentials "$ROOT"
rm -f "${ROOT}/secrets/server.secret"
unset MYNET_SERVER_SECRET || true

echo "Fresh staging: coordinator database and staging secrets removed."
echo "Next: ./scripts/run-acceptance.sh (or gen-ca, compose up, bootstrap, enroll-three)."
