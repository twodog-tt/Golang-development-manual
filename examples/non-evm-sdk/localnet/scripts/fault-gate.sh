#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=common.sh
source "$SCRIPT_DIR/common.sh"

requested_chain=${1:-}
validate_chain "$requested_chain"
load_chain_manifest "$requested_chain"

"$SCRIPT_DIR/toxiproxy.sh" up
trap '"$SCRIPT_DIR/toxiproxy.sh" reset >/dev/null 2>&1 || true' EXIT

identity_path=$(identity_file "$requested_chain")
if [[ -f "$identity_path" ]]; then
  "$SCRIPT_DIR/probe.sh" wait "$requested_chain" "$DIRECT_ENDPOINT" check
else
  "$SCRIPT_DIR/node.sh" status "$requested_chain" >/dev/null
  "$SCRIPT_DIR/probe.sh" wait "$requested_chain" "$DIRECT_ENDPOINT" capture
fi
"$SCRIPT_DIR/probe.sh" wait "$requested_chain" "$PROXY_ENDPOINT" check
"$SCRIPT_DIR/gate.sh" "$requested_chain" baseline proxy

for injected_fault in latency timeout reset; do
  printf 'injecting %s into %s\n' "$injected_fault" "$requested_chain"
  "$SCRIPT_DIR/toxiproxy.sh" apply "$requested_chain" "$injected_fault"
  "$SCRIPT_DIR/gate.sh" "$requested_chain" fault proxy
  "$SCRIPT_DIR/toxiproxy.sh" reset
  "$SCRIPT_DIR/probe.sh" wait "$requested_chain" "$PROXY_ENDPOINT" check
done

"$SCRIPT_DIR/gate.sh" "$requested_chain" baseline proxy
printf '%s passed latency, timeout, reset, and recovery gates\n' "$requested_chain"
