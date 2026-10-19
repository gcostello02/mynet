#!/usr/bin/env bash
# Operator-reviewed M2 real-network staging. Does NOT modify production mesh.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib-compose.sh
source "${ROOT}/scripts/lib-compose.sh"
staging_compose_init "$ROOT"

if [[ "${MYNET_STAGING_I_UNDERSTAND:-}" != "1" ]]; then
  cat <<'EOF'
MyNet M2 real-network staging requires Docker with privileged containers (NET_ADMIN, /dev/net/tun).

This script does NOT touch production coordinator, Tailscale, or host routes.
Set MYNET_STAGING_I_UNDERSTAND=1 to continue.

Mini PC: documentation indicates production mynet-server runs at ~/mynet on the
home mini PC behind mesh.example.com — this script never targets that host.
EOF
  exit 1
fi

mkdir -p "${ROOT}/artifacts"
trap 'echo "acceptance failed at $(date -u +%Y-%m-%dT%H:%M:%SZ); see artifacts/last-error.txt" | tee "${ROOT}/artifacts/last-error.txt"' ERR

staging_load_server_secret "$ROOT"

"${ROOT}/scripts/gen-ca.sh"
docker compose build
docker compose up -d coordinator proxy
"${ROOT}/scripts/bootstrap-coordinator.sh"
"${ROOT}/scripts/enroll-three.sh"

echo "Starting privileged client containers (profile: clients)..."
docker compose --profile clients up -d client-a client-b client-c

sleep 20
"${ROOT}/scripts/data-plane.sh"
"${ROOT}/scripts/recovery.sh"
"${ROOT}/scripts/revoke.sh"

echo "Acceptance finished. Artifacts in deploy/staging/artifacts/"
echo "Teardown: deploy/staging/scripts/teardown.sh"
