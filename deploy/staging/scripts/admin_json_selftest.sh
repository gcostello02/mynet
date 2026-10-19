#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib-admin.sh
source "${ROOT}/scripts/lib-admin.sh"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

valid='[{"device_id":"a","name":"x","revoked":false}]'
printf '%s' "$valid" >"$tmpdir/valid.json"

parse_devices_json() {
  local file="$1"
  jq -e . "$file" >/dev/null
}

parse_devices_json "$tmpdir/valid.json"

polluted=$'time="2026-01-01T00:00:00Z" level=warning msg=foo\n[{"device_id":"a","revoked":true}]'
if printf '%s' "$polluted" | jq -e . >/dev/null 2>&1; then
  echo "polluted docker line should not parse as whole document" >&2
  exit 1
fi

if printf '%s' "$polluted" | tail -1 | jq -e . >/dev/null 2>&1; then
  echo "last line extraction ok"
else
  echo "expected json on last line" >&2
  exit 1
fi

if jq -e . /dev/null 2>/dev/null; then
  echo "empty should fail" >&2
  exit 1
fi

# Harness helper simulation (no docker)
staging_admin_devices_json() {
  cat "$tmpdir/valid.json"
}
staging_admin_device_revoked() {
  local _root="$1" id="$2"
  staging_admin_devices_json "$1" | jq -e --arg id "$id" '.[] | select(.device_id == $id and .revoked == true)' >/dev/null
}

if staging_admin_device_revoked "$ROOT" "missing"; then
  echo "missing device should not be revoked" >&2
  exit 1
fi

printf '%s' '[{"device_id":"a","revoked":true}]' >"$tmpdir/valid.json"
staging_admin_device_revoked "$ROOT" "a" || exit 1

echo "admin_json_selftest: OK"
