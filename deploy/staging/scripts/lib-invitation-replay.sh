#!/usr/bin/env bash
# Classification helpers for invitation-replay.sh (sourced, not executed directly).

# Redact long base64-like invitation lines from log output.
invitation_replay_sanitize_log() {
  { echo "$1" | grep -vE '^[A-Za-z0-9+/_=.-]{40,}$' || true; } | head -30
}

# classify_replay_join_result <output> <exit_code>
# Prints one of: success | coordinator_rejected | docker_compose_error | local_validation |
#   tls_connectivity | auth_denied | unexpected
classify_replay_join_result() {
  local out="$1" ec="$2"
  if [[ "$ec" -eq 0 ]]; then
    echo "success"
    return
  fi
  if echo "$out" | grep -qiE 'unknown flag:.*tmpfs|unknown flag:.*compose|no such service'; then
    echo "docker_compose_error"
    return
  fi
  if echo "$out" | grep -qiE 'supervised daemon is running|device identity already exists|device name is required'; then
    echo "local_validation"
    return
  fi
  if echo "$out" | grep -qiE 'x509:|certificate|tls:|coordinator URL must use https|failed to verify'; then
    echo "tls_connectivity"
    return
  fi
  if echo "$out" | grep -qiE 'http 410|invitation uses exhausted|enrollment denied|uses exhausted'; then
    echo "coordinator_rejected"
    return
  fi
  if echo "$out" | grep -qiE 'http 401|http 403|signature verification failed'; then
    echo "auth_denied"
    return
  fi
  if echo "$out" | grep -qiE 'connection refused|no such host|network is unreachable|timeout|http 000'; then
    echo "tls_connectivity"
    return
  fi
  echo "unexpected"
}
