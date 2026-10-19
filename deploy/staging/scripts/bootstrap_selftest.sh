#!/usr/bin/env bash
# Offline checks for bootstrap response parsing (no Docker).
set -euo pipefail

parse_bootstrap_response() {
  local raw="$1"
  local code body
  code="$(echo "$raw" | tail -1 | sed 's/__HTTP_STATUS__//')"
  body="$(echo "$raw" | sed '$d')"
  echo "code=$code"
  echo "body_len=${#body}"
}

sample_ok="$(printf '%s\n__HTTP_STATUS__201\n' '{"admin_id":"a1"}')"
out="$(parse_bootstrap_response "$sample_ok")"
echo "$out" | grep -q 'code=201' || exit 1

sample_already="$(printf '%s\n__HTTP_STATUS__403\n' 'setup already completed')"
out="$(parse_bootstrap_response "$sample_already")"
echo "$out" | grep -q 'code=403' || exit 1

echo "bootstrap_selftest: OK"
