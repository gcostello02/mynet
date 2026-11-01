#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib-invitation-replay.sh
source "${ROOT}/scripts/lib-invitation-replay.sh"

if ! grep -q 'invitation-replay:' "${ROOT}/docker-compose.yml"; then
  echo "missing invitation-replay service in docker-compose.yml" >&2
  exit 1
fi
if ! grep -A20 'invitation-replay:' "${ROOT}/docker-compose.yml" | grep -q 'tmpfs:'; then
  echo "invitation-replay service must define tmpfs in compose (not compose run --tmpfs)" >&2
  exit 1
fi

if [[ "$(classify_replay_join_result 'unknown flag: --tmpfs' 1)" != "docker_compose_error" ]]; then
  echo "tmpfs flag misclassification" >&2
  exit 1
fi

out='enroll failed: invitation uses exhausted (http 410)'
if [[ "$(classify_replay_join_result "$out" 1)" != "coordinator_rejected" ]]; then
  echo "410 exhausted misclassification" >&2
  exit 1
fi

out='enroll failed: enrollment denied (http 410)'
if [[ "$(classify_replay_join_result "$out" 1)" != "coordinator_rejected" ]]; then
  echo "410 enrollment denied misclassification" >&2
  exit 1
fi

if [[ "$(classify_replay_join_result 'device identity already exists' 1)" != "local_validation" ]]; then
  echo "local validation misclassification" >&2
  exit 1
fi

if [[ "$(classify_replay_join_result 'x509: certificate signed by unknown authority' 1)" != "tls_connectivity" ]]; then
  echo "tls misclassification" >&2
  exit 1
fi

if [[ "$(classify_replay_join_result 'ok' 0)" != "success" ]]; then
  echo "success misclassification" >&2
  exit 1
fi

secret='eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.abcdefghijklmnopqrstuvwxyz0123456789'
sanitized="$(invitation_replay_sanitize_log "$secret")"
if echo "$sanitized" | grep -q 'eyJhbGci'; then
  echo "sanitizer should drop long invitation-like lines" >&2
  exit 1
fi

# compose run line must not pass --tmpfs after service name (grep script source)
if grep -E 'admin-toolbox.*--tmpfs|--tmpfs.*-lc' "${ROOT}/scripts/invitation-replay.sh" 2>/dev/null; then
  echo "invitation-replay.sh must not pass --tmpfs to container command" >&2
  exit 1
fi
if ! grep -q 'compose --profile tools run' "${ROOT}/scripts/invitation-replay.sh"; then
  echo "expected invitation-replay compose service run" >&2
  exit 1
fi

echo "invitation_replay_selftest: OK"
