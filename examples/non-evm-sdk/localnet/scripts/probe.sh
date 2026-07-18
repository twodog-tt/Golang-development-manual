#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=common.sh
source "$SCRIPT_DIR/common.sh"

action=${1:-}
requested_chain=${2:-}
endpoint=${3:-}
identity_mode=${4:-none}

[[ "$action" == "once" || "$action" == "wait" ]] || die "usage: probe.sh <once|wait> <chain> <endpoint> [none|capture|check]"
validate_chain "$requested_chain"
[[ -n "$endpoint" ]] || die "endpoint is required"
case "$identity_mode" in
  none|capture|check) ;;
  *) die "identity mode must be none, capture, or check" ;;
esac

require_command curl
require_command jq
load_chain_manifest "$requested_chain"

PROBED_IDENTITY=
PROBED_SUMMARY=

post_json() {
  local payload=$1
  curl --silent --show-error --fail --max-time "${LOCALNET_PROBE_REQUEST_TIMEOUT_SECONDS:-3}" \
    -H 'Accept: application/json' \
    -H 'Content-Type: application/json' \
    --data "$payload" \
    "$endpoint"
}

probe_once() {
  local response
  local health
  local height
  case "$requested_chain" in
    solana)
      health=$(post_json '{"jsonrpc":"2.0","id":1,"method":"getHealth"}') || return 1
      [[ "$(jq -r '.result // empty' <<<"$health")" == "ok" ]] || return 1
      response=$(post_json '{"jsonrpc":"2.0","id":1,"method":"getGenesisHash"}') || return 1
      PROBED_IDENTITY=$(jq -r '.result // empty' <<<"$response")
      [[ -n "$PROBED_IDENTITY" ]] || return 1
      PROBED_SUMMARY="health=ok genesis_hash=$PROBED_IDENTITY"
      ;;
    cosmos)
      response=$(post_json '{"jsonrpc":"2.0","id":1,"method":"status"}') || return 1
      [[ "$(jq -r '.result.sync_info.catching_up' <<<"$response")" == "false" ]] || return 1
      height=$(jq -r '.result.sync_info.latest_block_height // empty' <<<"$response")
      [[ "$height" =~ ^[0-9]+$ && "$height" -gt 0 ]] || return 1
      PROBED_IDENTITY=$(jq -r '.result.node_info.network // empty' <<<"$response")
      [[ -n "$PROBED_IDENTITY" ]] || return 1
      PROBED_SUMMARY="height=$height chain_id=$PROBED_IDENTITY"
      ;;
    aptos)
      response=$(curl --silent --show-error --fail --max-time "${LOCALNET_PROBE_REQUEST_TIMEOUT_SECONDS:-3}" \
        -H 'Accept: application/json' "$endpoint") || return 1
      PROBED_IDENTITY=$(jq -r '.chain_id // empty | tostring' <<<"$response")
      height=$(jq -r '.block_height // empty | tostring' <<<"$response")
      [[ -n "$PROBED_IDENTITY" && "$height" =~ ^[0-9]+$ ]] || return 1
      PROBED_SUMMARY="block_height=$height chain_id=$PROBED_IDENTITY"
      ;;
    sui)
      response=$(post_json '{"operationName":"Readiness","query":"query Readiness { chainIdentifier checkpoint { sequenceNumber digest } }"}') || return 1
      [[ "$(jq '.errors // [] | length' <<<"$response")" == "0" ]] || return 1
      PROBED_IDENTITY=$(jq -r '.data.chainIdentifier // empty' <<<"$response")
      height=$(jq -r '.data.checkpoint.sequenceNumber // empty | tostring' <<<"$response")
      [[ -n "$PROBED_IDENTITY" && "$height" =~ ^[0-9]+$ ]] || return 1
      PROBED_SUMMARY="checkpoint=$height chain_identifier=$PROBED_IDENTITY transport=GraphQL"
      ;;
  esac
}

if [[ "$action" == "once" ]]; then
  probe_once || die "$requested_chain readiness probe failed at $endpoint"
else
  case "$requested_chain" in
    solana|cosmos) timeout_seconds=${LOCALNET_READY_TIMEOUT_SECONDS:-90} ;;
    aptos) timeout_seconds=${LOCALNET_READY_TIMEOUT_SECONDS:-180} ;;
    sui) timeout_seconds=${LOCALNET_READY_TIMEOUT_SECONDS:-300} ;;
  esac
  deadline=$((SECONDS + timeout_seconds))
  until probe_once; do
    if [[ -n "${LOCALNET_WATCH_PID:-}" ]] && ! kill -0 "$LOCALNET_WATCH_PID" 2>/dev/null; then
      die "$requested_chain node process $LOCALNET_WATCH_PID exited before readiness"
    fi
    (( SECONDS < deadline )) || die "$requested_chain did not become ready within ${timeout_seconds}s"
    sleep 1
  done
fi

if [[ -n "$EXPECTED_IDENTITY" && "$PROBED_IDENTITY" != "$EXPECTED_IDENTITY" ]]; then
  die "$requested_chain identity $PROBED_IDENTITY does not match manifest identity $EXPECTED_IDENTITY"
fi

identity_path=$(identity_file "$requested_chain")
case "$identity_mode" in
  capture)
    mkdir -p "$(dirname "$identity_path")"
    printf '%s\n' "$PROBED_IDENTITY" > "$identity_path"
    ;;
  check)
    [[ -f "$identity_path" ]] || die "missing captured identity for $requested_chain"
    trusted_identity=$(tr -d '\r\n' < "$identity_path")
    [[ -n "$trusted_identity" ]] || die "captured identity for $requested_chain is empty"
    [[ "$PROBED_IDENTITY" == "$trusted_identity" ]] || die "$requested_chain identity changed: got $PROBED_IDENTITY, expected $trusted_identity"
    ;;
esac

printf '%s ready: %s\n' "$requested_chain" "$PROBED_SUMMARY"

