#!/usr/bin/env bash
# Waits for invitation file, joins, starts supervised daemon + VPN. Staging-only.
set -euo pipefail

update-ca-certificates 2>/dev/null || true

NAME="${MYNET_CLIENT_NAME:?}"
STATE="${MYNET_STATE_DIR:-/var/lib/mynet}"
export MYNET_STATE_DIR="$STATE"
INV_FILE="/artifacts/inv-${NAME}.txt"
READY_FILE="/artifacts/${NAME}.vpn-ready"
FAIL_FILE="/artifacts/${NAME}.failed"

rm -f "$READY_FILE" "$FAIL_FILE"

echo "staging client ${NAME} waiting for invitation at ${INV_FILE}..."
for i in $(seq 1 120); do
  [[ -f "$INV_FILE" ]] && break
  sleep 1
done
if [[ ! -f "$INV_FILE" ]]; then
  echo "invitation not found" | tee "$FAIL_FILE"
  exit 1
fi

if [[ ! -f "${STATE}/identity.json" ]]; then
  if ! mynet join --name "$NAME" "$(tr -d '\n' <"$INV_FILE")"; then
    echo "join failed" | tee "$FAIL_FILE"
    exit 1
  fi
fi

if ! pgrep -f 'mynet _daemon' >/dev/null 2>&1; then
  mynet _daemon &
  DAEMON_PID=$!
  for i in $(seq 1 30); do
    if mynet status >/dev/null 2>&1; then
      break
    fi
    if ! kill -0 "$DAEMON_PID" 2>/dev/null; then
      echo "daemon exited during startup" | tee "$FAIL_FILE"
      exit 1
    fi
    sleep 1
  done
else
  DAEMON_PID="$(pgrep -f 'mynet _daemon' | head -1)"
fi

if ! mynet up; then
  echo "mynet up failed" | tee "$FAIL_FILE"
  exit 1
fi

touch "$READY_FILE"
echo "staging client ${NAME} VPN up (daemon pid ${DAEMON_PID})"

if ! wait "$DAEMON_PID"; then
  echo "daemon exited unexpectedly" | tee "$FAIL_FILE"
  exit 1
fi

echo "daemon stopped" | tee "$FAIL_FILE"
exit 1
