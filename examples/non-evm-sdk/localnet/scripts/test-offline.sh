#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=common.sh
source "$SCRIPT_DIR/common.sh"

requested_chain=${1:-}
if [[ -n "$requested_chain" ]]; then
  validate_chain "$requested_chain"
  chains=("$requested_chain")
else
  chains=(solana cosmos aptos sui)
fi

unset NON_EVM_LOCALNET
for current_chain in "${chains[@]}"; do
  printf 'offline adapter gate: %s\n' "$current_chain"
  (
    cd "$SDK_ROOT/$current_chain"
    GOPROXY=off GOSUMDB=off run_go test -count=1 ./...
  )
done

