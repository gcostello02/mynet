#!/usr/bin/env bash
# Shared compose context for staging scripts (project-scoped only).
set -euo pipefail

staging_compose_init() {
  local root="$1"
  cd "$root"
  export COMPOSE_PROJECT_NAME=mynet-m2-staging
}

# Persist coordinator secret across partial runs (never print).
# File wins over shell env unless MYNET_STAGING_ALLOW_ENV_SECRET=1.
staging_load_server_secret() {
  local root="$1"
  local f="${root}/secrets/server.secret"
  mkdir -p "${root}/secrets"
  if [[ -f "$f" ]]; then
    if [[ -n "${MYNET_SERVER_SECRET:-}" && "${MYNET_STAGING_ALLOW_ENV_SECRET:-}" != "1" ]]; then
      echo "staging: using secrets/server.secret (unset env or set MYNET_STAGING_ALLOW_ENV_SECRET=1 to override)" >&2
    fi
    MYNET_SERVER_SECRET="$(tr -d '\n' <"$f")"
  else
    if [[ -z "${MYNET_SERVER_SECRET:-}" ]]; then
      MYNET_SERVER_SECRET="$(openssl rand -base64 32 | tr -d '\n')"
    fi
    umask 077
    printf '%s' "$MYNET_SERVER_SECRET" >"$f"
    chmod 600 "$f"
  fi
  export MYNET_SERVER_SECRET
  export MYNET_ENROLLMENT_SECRET="${MYNET_ENROLLMENT_SECRET:-staging-enrollment-secret-not-for-prod}"
}

staging_clear_local_credentials() {
  local root="$1"
  rm -rf "${root}/admin-state"
  rm -f "${root}/artifacts/inv-"*.txt "${root}/artifacts/admin-session.json" 2>/dev/null || true
}

staging_die() {
  echo "staging error: $*" >&2
  exit 1
}
