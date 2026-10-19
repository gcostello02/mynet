#!/usr/bin/env bash
set -euo pipefail

# admin-toolbox service sets entrypoint to /bin/bash — pass only bash flags/args (not "bash" again).
staging_admin() {
  local ROOT="$1"
  shift
  local quoted=""
  if (($# > 0)); then
    printf -v quoted '%q ' "$@"
  fi
  (
    cd "$ROOT"
    export COMPOSE_PROJECT_NAME=mynet-m2-staging
    docker compose run --rm --no-deps admin-toolbox \
      -lc "update-ca-certificates >/dev/null 2>&1; exec mynet admin ${quoted}"
  )
}
