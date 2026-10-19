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

staging_admin_env() {
  : "${MYNET_STAGING_ADMIN_USER:=staging-admin}"
  : "${MYNET_STAGING_ADMIN_PASS:=staging-password-12chars}"
  : "${MYNET_COORD_PUBLIC_URL:=https://staging-coord.mynet.test}"
  export MYNET_STAGING_ADMIN_USER MYNET_STAGING_ADMIN_PASS MYNET_COORD_PUBLIC_URL
}

# Fresh login for the configured staging operator (password from env, never logged).
staging_admin_login() {
  local ROOT="$1"
  staging_admin_env
  if [[ -z "${MYNET_STAGING_ADMIN_PASS}" ]]; then
    echo "staging error: MYNET_STAGING_ADMIN_PASS is not set" >&2
    return 1
  fi
  printf '%s\n' "$MYNET_STAGING_ADMIN_PASS" | staging_admin "$ROOT" login \
    --server "$MYNET_COORD_PUBLIC_URL" --user "$MYNET_STAGING_ADMIN_USER"
}

_staging_admin_run_once() {
  local ROOT="$1"
  local tmp_out="$2"
  shift 2
  staging_admin "$ROOT" "$@" >"$tmp_out" 2>&1
}

# Run an admin subcommand; on HTTP 401 re-login as the same user and retry once.
staging_admin_run() {
  local ROOT="$1"
  shift
  local tmp_out ec
  tmp_out="$(mktemp)"
  if ! _staging_admin_run_once "$ROOT" "$tmp_out" "$@"; then
    ec=$?
    if grep -qE 'http 401|admin authentication required' "$tmp_out"; then
      echo "staging: admin session invalid; requesting fresh login for ${MYNET_STAGING_ADMIN_USER:-staging-admin}" >&2
      staging_admin_env
      if ! staging_admin_login "$ROOT" >/dev/null 2>&1; then
        echo "staging error: admin re-login failed (check credentials)" >&2
        cat "$tmp_out" >&2
        rm -f "$tmp_out"
        return 1
      fi
      if _staging_admin_run_once "$ROOT" "$tmp_out" "$@"; then
        cat "$tmp_out"
        rm -f "$tmp_out"
        return 0
      fi
      ec=$?
    fi
    cat "$tmp_out" >&2
    rm -f "$tmp_out"
    return "$ec"
  fi
  cat "$tmp_out"
  rm -f "$tmp_out"
  return 0
}

# Exit non-zero unless admin session works (re-login on 401).
staging_admin_ensure() {
  local ROOT="$1"
  staging_admin_env
  if ! staging_admin_run "$ROOT" status >/dev/null; then
    echo "staging error: admin authentication failed" >&2
    return 1
  fi
}
