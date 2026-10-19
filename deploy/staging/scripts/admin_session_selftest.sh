#!/usr/bin/env bash
# Offline checks for admin session refresh heuristics (no Docker).
set -euo pipefail

sample_401=$'admin authentication required (http 401)'
if ! echo "$sample_401" | grep -qE 'http 401|admin authentication required'; then
  echo "401 pattern match failed" >&2
  exit 1
fi

sample_ok=$'{"admin_id":"x"}'
if echo "$sample_ok" | grep -qE 'http 401|admin authentication required'; then
  echo "false positive on ok output" >&2
  exit 1
fi

echo "admin_session_selftest: OK"
