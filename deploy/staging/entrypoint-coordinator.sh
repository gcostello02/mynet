#!/usr/bin/env bash
set -euo pipefail
exec /usr/local/bin/mynet-server --listen "${MYNET_LISTEN:-:8080}" --data "${MYNET_DATA_PATH}"
