#!/usr/bin/env bash
# Shared compose context for staging scripts (project-scoped only).
set -euo pipefail

staging_root() {
  cd "$(dirname "${BASH_SOURCE[1]}")/.." && pwd
}

staging_compose_init() {
  local root="$1"
  cd "$root"
  export COMPOSE_PROJECT_NAME=mynet-m2-staging
}

# Persist coordinator secret across partial runs (never print).
staging_load_server_secret() {
  local root="$1"
  local f="${root}/secrets/server.secret"
  mkdir -p "${root}/secrets"
  if [[ -f "$f" ]]; then
    MYNET_SERVER_SECRET="$(tr -d '\n' <"$f")"
  else
    MYNET_SERVER_SECRET="${MYNET_SERVER_SECRET:-$(openssl rand -base64 32 | tr -d '\n')}"
    printf '%s' "$MYNET_SERVER_SECRET" >"$f"
    chmod 600 "$f"
  fi
  export MYNET_SERVER_SECRET
  export MYNET_ENROLLMENT_SECRET="${MYNET_ENROLLMENT_SECRET:-staging-enrollment-secret-not-for-prod}"
}

staging_die() {
  echo "staging error: $*" >&2
  exit 1
}
