#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=common.sh
source "$SCRIPT_DIR/common.sh"

action=${1:-}
requested_chain=${2:-}
fault=${3:-}

compose() {
  require_command docker
  docker compose -f "$LOCALNET_ROOT/compose.yaml" "$@"
}

wait_for_admin() {
  local deadline=$((SECONDS + 60))
  until curl --silent --show-error --fail --max-time 2 "$TOXIPROXY_ADMIN/version" >/dev/null 2>&1; do
    (( SECONDS < deadline )) || die "Toxiproxy did not become ready"
    sleep 1
  done
}

verify_server_version() {
  require_command jq
  # shellcheck source=/dev/null
  source "$LOCALNET_ROOT/manifests/toxiproxy.env"
  local actual
  actual=$(
    curl --silent --show-error --fail "$TOXIPROXY_ADMIN/version" |
      jq -r 'if type == "object" then .version else . end' |
      sed 's/^v//'
  )
  [[ "$actual" == "$TOXIPROXY_VERSION" ]] || die "Toxiproxy version $actual, expected $TOXIPROXY_VERSION"
}

reset_all() {
  curl --silent --show-error --fail -X POST "$TOXIPROXY_ADMIN/reset" >/dev/null
}

apply_fault() {
  validate_chain "$requested_chain"
  load_chain_manifest "$requested_chain"
  reset_all

  local payload
  case "$fault" in
    latency)
      payload='{"name":"compat-latency","type":"latency","stream":"downstream","toxicity":1.0,"attributes":{"latency":1500,"jitter":0}}'
      ;;
    timeout)
      payload='{"name":"compat-timeout","type":"timeout","stream":"downstream","toxicity":1.0,"attributes":{"timeout":0}}'
      ;;
    reset)
      payload='{"name":"compat-reset","type":"reset_peer","stream":"downstream","toxicity":1.0,"attributes":{"timeout":0}}'
      ;;
    *)
      die "fault must be latency, timeout, or reset"
      ;;
  esac
  curl --silent --show-error --fail \
    -H 'Content-Type: application/json' \
    --data "$payload" \
    "$TOXIPROXY_ADMIN/proxies/$PROXY_NAME/toxics" >/dev/null
}

case "$action" in
  up)
    compose up -d toxiproxy
    wait_for_admin
    verify_server_version
    reset_all
    ;;
  down)
    compose down --remove-orphans
    ;;
  reset)
    wait_for_admin
    verify_server_version
    reset_all
    ;;
  apply)
    wait_for_admin
    verify_server_version
    apply_fault
    ;;
  version)
    wait_for_admin
    verify_server_version
    printf 'Toxiproxy version is pinned and ready\n'
    ;;
  *)
    die "usage: toxiproxy.sh <up|down|reset|apply|version> [chain] [latency|timeout|reset]"
    ;;
esac
