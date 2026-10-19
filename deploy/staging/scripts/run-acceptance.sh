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
rm -f "${ROOT}/artifacts/last-error.txt" "${ROOT}/artifacts/"*.failed "${ROOT}/artifacts/"*.vpn-ready 2>/dev/null || true

STAGE="init"
staging_fail() {
  local ec=$?
  {
    echo "acceptance failed"
    echo "stage: ${STAGE}"
    echo "exit_code: ${ec}"
    echo "line: ${BASH_LINENO[0]:-?}"
    echo "command: ${BASH_COMMAND:-?}"
    echo "time: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  } >"${ROOT}/artifacts/last-error.txt"
  cat "${ROOT}/artifacts/last-error.txt" >&2
  exit "$ec"
}
trap staging_fail ERR

staging_load_server_secret "$ROOT"

STAGE="gen-ca"
"${ROOT}/scripts/gen-ca.sh"
STAGE="docker-build"
docker compose build
STAGE="coordinator-up"
docker compose up -d coordinator proxy
STAGE="bootstrap"
"${ROOT}/scripts/bootstrap-coordinator.sh"
STAGE="enroll"
"${ROOT}/scripts/enroll-three.sh"

STAGE="clients-up"
echo "Starting privileged client containers (profile: clients)..."
docker compose --profile clients up -d client-a client-b client-c

STAGE="data-plane-wait"
sleep 20
STAGE="data-plane"
"${ROOT}/scripts/data-plane.sh"
STAGE="recovery"
"${ROOT}/scripts/recovery.sh"
STAGE="revoke"
"${ROOT}/scripts/revoke.sh"

trap - ERR
echo "Acceptance finished. Artifacts in deploy/staging/artifacts/"
echo "Teardown: deploy/staging/scripts/teardown.sh"
