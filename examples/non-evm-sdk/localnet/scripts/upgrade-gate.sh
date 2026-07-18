#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=common.sh
source "$SCRIPT_DIR/common.sh"

requested_chain=${1:-}
validate_chain "$requested_chain"
load_chain_manifest "$requested_chain"

"$SCRIPT_DIR/toxiproxy.sh" up
"$SCRIPT_DIR/toxiproxy.sh" reset
"$SCRIPT_DIR/node.sh" stop "$requested_chain"

printf '%s pre-upgrade gate: N-1=%s\n' "$requested_chain" "$N_MINUS_1_VERSION"
"$SCRIPT_DIR/node.sh" start "$requested_chain" n-1 fresh
"$SCRIPT_DIR/probe.sh" wait "$requested_chain" "$DIRECT_ENDPOINT" capture
"$SCRIPT_DIR/probe.sh" wait "$requested_chain" "$PROXY_ENDPOINT" check
"$SCRIPT_DIR/gate.sh" "$requested_chain" baseline proxy
old_identity=$(tr -d '\r\n' < "$(identity_file "$requested_chain")")

"$SCRIPT_DIR/node.sh" stop "$requested_chain"
printf '%s post-upgrade gate: N=%s state=%s\n' "$requested_chain" "$N_VERSION" "$UPGRADE_STATE_MODE"
"$SCRIPT_DIR/node.sh" start "$requested_chain" n "$UPGRADE_STATE_MODE"
if [[ "$UPGRADE_STATE_MODE" == "reuse" ]]; then
  printf '%s\n' "$old_identity" > "$(identity_file "$requested_chain")"
  "$SCRIPT_DIR/probe.sh" wait "$requested_chain" "$DIRECT_ENDPOINT" check
else
  "$SCRIPT_DIR/probe.sh" wait "$requested_chain" "$DIRECT_ENDPOINT" capture
fi
"$SCRIPT_DIR/probe.sh" wait "$requested_chain" "$PROXY_ENDPOINT" check
"$SCRIPT_DIR/gate.sh" "$requested_chain" baseline proxy
"$SCRIPT_DIR/test-offline.sh" "$requested_chain"

printf '%s passed N-1=%s to N=%s endpoint upgrade gate\n' "$requested_chain" "$N_MINUS_1_VERSION" "$N_VERSION"

