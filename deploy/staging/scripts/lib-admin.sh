#!/usr/bin/env bash
set -euo pipefail

staging_admin() {
  local ROOT="$1"
  shift
  docker compose run --rm --no-deps \
    --entrypoint /bin/bash \
    -e MYNET_STATE_DIR=/admin-state \
    -v "${ROOT}/admin-state:/admin-state" \
    -v "${ROOT}/certs/ca.crt:/usr/local/share/ca-certificates/mynet-staging-ca.crt:ro" \
    --add-host "staging-coord.mynet.test:10.200.0.11" \
    coordinator \
    -lc "update-ca-certificates >/dev/null 2>&1; mynet admin $*"
}
