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
    docker compose run --rm --no-deps --quiet-pull admin-toolbox \
      -lc "update-ca-certificates >/dev/null 2>&1; exec mynet admin ${quoted}"
  )
}

staging_admin_env() {
  : "${MYNET_STAGING_ADMIN_USER:=staging-admin}"
  : "${MYNET_STAGING_ADMIN_PASS:=staging-password-12chars}"
  : "${MYNET_COORD_PUBLIC_URL:=https://staging-coord.mynet.test}"
  export MYNET_STAGING_ADMIN_USER MYNET_STAGING_ADMIN_PASS MYNET_COORD_PUBLIC_URL
}

staging_admin_login() {
  local ROOT="$1"
  staging_admin_env
  if [[ -z "${MYNET_STAGING_ADMIN_PASS}" ]]; then
    echo "staging error: MYNET_STAGING_ADMIN_PASS is not set" >&2
    return 1
  fi
  printf '%s\n' "$MYNET_STAGING_ADMIN_PASS" | staging_admin "$ROOT" login \
    --server "$MYNET_COORD_PUBLIC_URL" --user "$MYNET_STAGING_ADMIN_USER" 1>/dev/null
}

_staging_admin_capture() {
  local ROOT="$1"
  local out="$2"
  local err="$3"
  shift 3
  staging_admin "$ROOT" "$@" >"$out" 2>"$err"
}

# Run admin command; stdout is CLI output only (compose diagnostics go to err file).
staging_admin_run() {
  local ROOT="$1"
  shift
  local tmp_out tmp_err ec
  tmp_out="$(mktemp)"
  tmp_err="$(mktemp)"
  if ! _staging_admin_capture "$ROOT" "$tmp_out" "$tmp_err" "$@"; then
    ec=$?
    if grep -qE 'http 401|admin authentication required' "$tmp_err" || grep -qE 'http 401|admin authentication required' "$tmp_out"; then
      echo "staging: admin session invalid; requesting fresh login for ${MYNET_STAGING_ADMIN_USER:-staging-admin}" >&2
      staging_admin_env
      if ! staging_admin_login "$ROOT"; then
        echo "staging error: admin re-login failed (check credentials)" >&2
        head -20 "$tmp_err" >&2
        rm -f "$tmp_out" "$tmp_err"
        return 1
      fi
      if _staging_admin_capture "$ROOT" "$tmp_out" "$tmp_err" "$@"; then
        cat "$tmp_out"
        rm -f "$tmp_out" "$tmp_err"
        return 0
      fi
      ec=$?
    fi
    echo "staging error: admin command failed (exit ${ec})" >&2
    head -20 "$tmp_err" >&2
    head -20 "$tmp_out" >&2
    rm -f "$tmp_out" "$tmp_err"
    return "$ec"
  fi
  cat "$tmp_out"
  rm -f "$tmp_out" "$tmp_err"
  return 0
}

# Fetch devices list JSON; fails on empty output, invalid JSON, or nonzero CLI exit.
staging_admin_devices_json() {
  local ROOT="$1"
  local json err
  err="$(mktemp)"
  if ! json="$(staging_admin_run "$ROOT" devices list --format json 2>"$err")"; then
    echo "staging error: devices list command failed" >&2
    head -10 "$err" >&2
    rm -f "$err"
    return 1
  fi
  rm -f "$err"
  if [[ -z "${json//[[:space:]]/}" ]]; then
    echo "staging error: devices list returned empty output" >&2
    return 2
  fi
  if ! printf '%s' "$json" | jq -e . >/dev/null 2>&1; then
    echo "staging error: devices list is not valid JSON (harness parse error)" >&2
    printf '%s' "$json" | head -c 200 >&2
    echo >&2
    return 3
  fi
  printf '%s' "$json"
}

# Return 0 when device_id is revoked in coordinator JSON.
staging_admin_device_revoked() {
  local ROOT="$1" device_id="$2"
  local json
  json="$(staging_admin_devices_json "$ROOT")" || return $?
  if printf '%s' "$json" | jq -e --arg id "$device_id" '.[] | select(.device_id == $id and .revoked == true)' >/dev/null; then
    return 0
  fi
  return 1
}

staging_admin_ensure() {
  local ROOT="$1"
  staging_admin_env
  if ! staging_admin_run "$ROOT" status >/dev/null; then
    echo "staging error: admin authentication failed" >&2
    return 1
  fi
}
