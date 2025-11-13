#!/bin/sh

# Simple helper that ensures TLS certificates exist in the shared volume.
# If cert.pem and key.pem are missing, it generates a self-signed certificate.

set -eu

TLS_DIR="${TLS_DIR:-/var/lib/flotilla/tls}"
CERT_FILE="${TLS_CERT_FILE_PATH:-$TLS_DIR/cert.pem}"
KEY_FILE="${TLS_KEY_FILE_PATH:-$TLS_DIR/key.pem}"
TLS_SUBJECT="${TLS_SUBJECT:-/CN=flotilla.local}"
TLS_DAYS="${TLS_DAYS:-825}"

echo "tls-init: ensuring TLS assets in ${TLS_DIR}"

mkdir -p "${TLS_DIR}"

if [ -f "${CERT_FILE}" ] && [ -f "${KEY_FILE}" ]; then
  echo "tls-init: existing certificate and key detected, no action needed."
  exit 0
fi

echo "tls-init: generating self-signed certificate (subject=${TLS_SUBJECT}, days=${TLS_DAYS})"

openssl req -x509 -nodes -newkey rsa:4096 \
  -keyout "${KEY_FILE}" \
  -out "${CERT_FILE}" \
  -days "${TLS_DAYS}" \
  -subj "${TLS_SUBJECT}"

chmod 600 "${KEY_FILE}"
chmod 644 "${CERT_FILE}"

echo "tls-init: TLS assets ready."
