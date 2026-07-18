#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=common.sh
source "$SCRIPT_DIR/common.sh"

requested_chain=${1:-}
validate_chain "$requested_chain"
load_chain_manifest "$requested_chain"

state_dir=$(chain_state_dir "$requested_chain")
lane_file="$state_dir/lane"
[[ -f "$lane_file" ]] || die "$requested_chain has no recorded running lane"
recorded_lane=$(tr -d '\r\n' < "$lane_file")
validate_lane "$recorded_lane"

"$SCRIPT_DIR/toxiproxy.sh" up
"$SCRIPT_DIR/toxiproxy.sh" reset
"$SCRIPT_DIR/probe.sh" wait "$requested_chain" "$DIRECT_ENDPOINT" check
"$SCRIPT_DIR/gate.sh" "$requested_chain" baseline proxy

"$SCRIPT_DIR/node.sh" stop "$requested_chain"
"$SCRIPT_DIR/gate.sh" "$requested_chain" fault proxy

"$SCRIPT_DIR/node.sh" start "$requested_chain" "$recorded_lane" reuse
"$SCRIPT_DIR/probe.sh" wait "$requested_chain" "$DIRECT_ENDPOINT" check
"$SCRIPT_DIR/probe.sh" wait "$requested_chain" "$PROXY_ENDPOINT" check
"$SCRIPT_DIR/gate.sh" "$requested_chain" baseline proxy
printf '%s passed stop/unavailable/restart/recovery gates\n' "$requested_chain"

