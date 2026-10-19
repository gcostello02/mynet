#!/usr/bin/env bash
# Generate staging CA + coordinator TLS cert (operator-reviewed, local only).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CERT_DIR="${ROOT}/certs"
mkdir -p "$CERT_DIR"
CN="staging-coord.mynet.test"

if [[ -f "${CERT_DIR}/ca.crt" && -f "${CERT_DIR}/coord.crt" ]]; then
  echo "Certs already exist in ${CERT_DIR} (delete to regenerate)."
  exit 0
fi

openssl req -x509 -newkey rsa:3072 -days 825 -nodes \
  -keyout "${CERT_DIR}/ca.key" -out "${CERT_DIR}/ca.crt" \
  -subj "/CN=MyNet M2 Staging CA" 2>/dev/null

openssl req -newkey rsa:3072 -nodes \
  -keyout "${CERT_DIR}/coord.key" -out "${CERT_DIR}/coord.csr" \
  -subj "/CN=${CN}" 2>/dev/null

openssl x509 -req -in "${CERT_DIR}/coord.csr" \
  -CA "${CERT_DIR}/ca.crt" -CAkey "${CERT_DIR}/ca.key" -CAcreateserial \
  -out "${CERT_DIR}/coord.crt" -days 825 \
  -extfile <(printf "subjectAltName=DNS:%s\n" "$CN") 2>/dev/null

chmod 600 "${CERT_DIR}/ca.key" "${CERT_DIR}/coord.key"
echo "Wrote ${CERT_DIR}/ca.crt and coord.{crt,key}"
