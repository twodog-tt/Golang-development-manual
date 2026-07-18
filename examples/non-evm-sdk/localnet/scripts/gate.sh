#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=common.sh
source "$SCRIPT_DIR/common.sh"

requested_chain=${1:-}
mode=${2:-baseline}
endpoint_mode=${3:-proxy}
validate_chain "$requested_chain"
case "$mode" in
  baseline|fault) ;;
  *) die "gate mode must be baseline or fault" ;;
esac
case "$endpoint_mode" in
  direct|proxy) ;;
  *) die "endpoint mode must be direct or proxy" ;;
esac

load_chain_manifest "$requested_chain"
if [[ "$endpoint_mode" == "direct" ]]; then
  endpoint=$DIRECT_ENDPOINT
else
  endpoint=$PROXY_ENDPOINT
fi

identity_path=$(identity_file "$requested_chain")
[[ -f "$identity_path" ]] || die "capture a trusted $requested_chain identity before running the adapter gate"
trusted_identity=$(tr -d '\r\n' < "$identity_path")
[[ -n "$trusted_identity" ]] || die "$requested_chain identity file is empty"

export NON_EVM_LOCALNET=1
export NON_EVM_LOCALNET_HTTP_TIMEOUT_MS=${NON_EVM_LOCALNET_HTTP_TIMEOUT_MS:-5000}

case "$requested_chain" in
  solana)
    export SOLANA_LOCALNET_RPC=$endpoint
    export SOLANA_LOCALNET_EXPECTED_GENESIS=$trusted_identity
    baseline_test=TestLocalnetSolanaCompatibilityGate
    fault_test=TestLocalnetSolanaTransportFaultDoesNotBecomeState
    ;;
  cosmos)
    export COSMOS_LOCALNET_RPC=$endpoint
    export COSMOS_LOCALNET_EXPECTED_CHAIN_ID=$trusted_identity
    baseline_test=TestLocalnetCosmosCompatibilityGate
    fault_test=TestLocalnetCosmosTransportFaultDoesNotBecomeState
    ;;
  aptos)
    export APTOS_LOCALNET_REST=$endpoint
    export APTOS_LOCALNET_EXPECTED_CHAIN_ID=$trusted_identity
    baseline_test=TestLocalnetAptosCompatibilityGate
    fault_test=TestLocalnetAptosTransportFaultDoesNotBecomeState
    ;;
  sui)
    export SUI_LOCALNET_GRAPHQL=$endpoint
    export SUI_LOCALNET_EXPECTED_CHAIN_IDENTIFIER=$trusted_identity
    baseline_test=TestLocalnetSuiCompatibilityGate
    fault_test=TestLocalnetSuiTransportFaultDoesNotBecomeState
    ;;
esac

if [[ "$mode" == "baseline" ]]; then
  selected_test=$baseline_test
else
  selected_test=$fault_test
  export NON_EVM_LOCALNET_HTTP_TIMEOUT_MS=${NON_EVM_LOCALNET_FAULT_TIMEOUT_MS:-250}
fi

(
  cd "$SDK_ROOT/$requested_chain"
  GOPROXY=off GOSUMDB=off run_go test -count=1 -run "^${selected_test}$" .
)

