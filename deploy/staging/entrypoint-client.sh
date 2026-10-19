#!/usr/bin/env bash
# Waits for invitation file, joins, starts supervised daemon + VPN. Used only in staging containers.
set -euo pipefail

update-ca-certificates 2>/dev/null || true

NAME="${MYNET_CLIENT_NAME:?}"
STATE="${MYNET_STATE_DIR:-/var/lib/mynet}"
export MYNET_STATE_DIR="$STATE"
COORD="${MYNET_COORD_URL:?}"
INV_FILE="/artifacts/inv-${NAME}.txt"

echo "staging client ${NAME} waiting for invitation at ${INV_FILE}..."
for i in $(seq 1 120); do
  [[ -f "$INV_FILE" ]] && break
  sleep 1
done
if [[ ! -f "$INV_FILE" ]]; then
  echo "invitation not found" >&2
  exit 1
fi

if [[ ! -f "${STATE}/identity.json" ]]; then
  mynet join --name "$NAME" "$(tr -d '\n' <"$INV_FILE")"
fi

if ! pgrep -f 'mynet _daemon' >/dev/null 2>&1; then
  mynet _daemon &
  for i in $(seq 1 30); do
    mynet status >/dev/null 2>&1 && break
    sleep 1
  done
fi

mynet up || true

echo "staging client ${NAME} ready; holding container"
tail -f /dev/null
