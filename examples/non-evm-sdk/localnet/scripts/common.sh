#!/usr/bin/env bash

set -euo pipefail

LOCALNET_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
SDK_ROOT=$(cd "$LOCALNET_ROOT/.." && pwd)
STATE_ROOT=${LOCALNET_STATE_ROOT:-"$LOCALNET_ROOT/.state"}
BIN_ROOT=${LOCALNET_BIN_ROOT:-"$LOCALNET_ROOT/bin"}
CACHE_ROOT=${LOCALNET_CACHE_ROOT:-"$LOCALNET_ROOT/.cache"}
TOXIPROXY_ADMIN=${TOXIPROXY_ADMIN:-http://127.0.0.1:8474}

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "required command is missing: $1"
}

sha256_file() {
  local file=$1
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
  else
    die "sha256sum or shasum is required"
  fi
}

validate_chain() {
  case "${1:-}" in
    solana|cosmos|aptos|sui) ;;
    *) die "chain must be one of: solana, cosmos, aptos, sui" ;;
  esac
}

validate_lane() {
  case "${1:-}" in
    n|n-1) ;;
    *) die "lane must be n or n-1" ;;
  esac
}

load_chain_manifest() {
  local requested_chain=${1:-}
  validate_chain "$requested_chain"
  # shellcheck source=/dev/null
  source "$LOCALNET_ROOT/manifests/$requested_chain.env"
}

select_lane() {
  local requested_lane=${1:-}
  validate_lane "$requested_lane"
  if [[ "$requested_lane" == "n" ]]; then
    SELECTED_VERSION=$N_VERSION
    SELECTED_TAG=$N_TAG
    SELECTED_COMMIT=$N_COMMIT
  else
    SELECTED_VERSION=$N_MINUS_1_VERSION
    SELECTED_TAG=$N_MINUS_1_TAG
    SELECTED_COMMIT=$N_MINUS_1_COMMIT
  fi
  SELECTED_LANE=$requested_lane
  SELECTED_BINARY="$BIN_ROOT/$CHAIN/$requested_lane/$BINARY_NAME"
}

go_binary() {
  if [[ -n "${GO_BIN:-}" ]]; then
    printf '%s\n' "$GO_BIN"
  elif [[ -x /opt/homebrew/bin/go ]]; then
    printf '%s\n' /opt/homebrew/bin/go
  else
    command -v go || die "Go is required"
  fi
}

run_go() {
  local binary
  binary=$(go_binary)
  env -u GOROOT "$binary" "$@"
}

chain_state_dir() {
  printf '%s/%s\n' "$STATE_ROOT" "$1"
}

identity_file() {
  printf '%s/identity\n' "$(chain_state_dir "$1")"
}

binary_version_output() {
  local requested_chain=$1
  local binary=$2
  if [[ "$requested_chain" == "cosmos" ]]; then
    "$binary" version
  else
    "$binary" --version
  fi
}

verify_binary() {
  local requested_chain=$1
  local requested_lane=$2
  load_chain_manifest "$requested_chain"
  select_lane "$requested_lane"
  [[ -x "$SELECTED_BINARY" ]] || die "missing $SELECTED_BINARY; run: LOCALNET_ALLOW_NETWORK=1 scripts/build-node.sh $requested_chain $requested_lane"

  local provenance="$SELECTED_BINARY.provenance"
  if [[ ! -f "$provenance" ]]; then
    if [[ "${LOCALNET_ALLOW_UNVERIFIED_BINARY:-0}" != "1" ]]; then
      die "missing provenance for $SELECTED_BINARY; refusing an unpinned node binary"
    fi
    printf 'warning: accepting an unverified node binary because LOCALNET_ALLOW_UNVERIFIED_BINARY=1\n' >&2
    binary_version_output "$requested_chain" "$SELECTED_BINARY" >&2
    return
  fi

  local provenance_chain=
  local provenance_lane=
  local provenance_tag=
  local provenance_commit=
  local provenance_sha256=
  while IFS='=' read -r key value; do
    case "$key" in
      chain) provenance_chain=$value ;;
      lane) provenance_lane=$value ;;
      tag) provenance_tag=$value ;;
      commit) provenance_commit=$value ;;
      sha256) provenance_sha256=$value ;;
    esac
  done < "$provenance"

  [[ "$provenance_chain" == "$requested_chain" ]] || die "binary provenance chain mismatch"
  [[ "$provenance_lane" == "$requested_lane" ]] || die "binary provenance lane mismatch"
  [[ "$provenance_tag" == "$SELECTED_TAG" ]] || die "binary provenance tag mismatch"
  [[ "$provenance_commit" == "$SELECTED_COMMIT" ]] || die "binary provenance commit mismatch"
  [[ "$provenance_sha256" =~ ^[0-9a-f]{64}$ ]] || die "binary provenance SHA-256 is missing"
  local actual_sha256
  actual_sha256=$(sha256_file "$SELECTED_BINARY")
  [[ "$actual_sha256" == "$provenance_sha256" ]] || die "binary SHA-256 does not match provenance"
  binary_version_output "$requested_chain" "$SELECTED_BINARY" >&2
}
