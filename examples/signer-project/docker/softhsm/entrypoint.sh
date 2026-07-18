#!/bin/sh
set -eu

: "${SOFTHSM2_CONF:=/etc/softhsm/softhsm2.conf}"
: "${SOFTHSM2_TOKEN_DIR:=/var/lib/softhsm/tokens}"
: "${PKCS11_TOKEN_LABEL:=signer-project}"
: "${PKCS11_SO_PIN:=12345678}"
: "${PKCS11_USER_PIN:=123456}"
: "${SIGNER_DB:=/var/lib/signer/fence.db}"

umask 077
mkdir -p "$SOFTHSM2_TOKEN_DIR" "$(dirname "$SOFTHSM2_CONF")" "$(dirname "$SIGNER_DB")"
printf '%s\n' \
  "directories.tokendir = $SOFTHSM2_TOKEN_DIR" \
  "objectstore.backend = file" \
  "log.level = INFO" \
  "slots.removable = false" > "$SOFTHSM2_CONF"
export SOFTHSM2_CONF

if ! softhsm2-util --show-slots | grep -F "Label:            $PKCS11_TOKEN_LABEL" >/dev/null 2>&1; then
  softhsm2-util --init-token --free \
    --label "$PKCS11_TOKEN_LABEL" \
    --so-pin "$PKCS11_SO_PIN" \
    --pin "$PKCS11_USER_PIN"
fi

if [ -z "${PKCS11_MODULE_PATH:-}" ]; then
  PKCS11_MODULE_PATH="$(find /usr/lib -name libsofthsm2.so -print -quit)"
fi
if [ -z "$PKCS11_MODULE_PATH" ]; then
  echo "SoftHSM2 PKCS#11 module was not found" >&2
  exit 1
fi
export PKCS11_MODULE_PATH

exec /usr/local/bin/pkcs11-demo "$@"
