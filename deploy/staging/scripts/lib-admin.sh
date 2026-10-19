#!/usr/bin/env bash
set -euo pipefail

# Admin CLI via admin-toolbox service (extra_hosts in compose; Compose v5 has no `compose run --add-host`).
staging_admin() {
  local ROOT="$1"
  shift
  (
    cd "$ROOT"
    export COMPOSE_PROJECT_NAME=mynet-m2-staging
    docker compose run --rm --no-deps admin-toolbox \
      bash -lc 'update-ca-certificates >/dev/null 2>&1; exec mynet admin "$@"' admin "$@"
  )
}
