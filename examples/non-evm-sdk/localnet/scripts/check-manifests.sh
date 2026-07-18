#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=common.sh
source "$SCRIPT_DIR/common.sh"

require_command git
require_command jq

for requested_chain in solana cosmos aptos sui; do
  load_chain_manifest "$requested_chain"
  [[ "$CHAIN" == "$requested_chain" ]] || die "$requested_chain manifest has CHAIN=$CHAIN"
  [[ -n "$SOURCE_REPOSITORY" && -n "$BINARY_NAME" ]] || die "$requested_chain manifest is incomplete"
  [[ "$N_COMMIT" =~ ^[0-9a-f]{40}$ ]] || die "$requested_chain N commit is not a full SHA"
  [[ "$N_MINUS_1_COMMIT" =~ ^[0-9a-f]{40}$ ]] || die "$requested_chain N-1 commit is not a full SHA"
  [[ "$N_COMMIT" != "$N_MINUS_1_COMMIT" ]] || die "$requested_chain lanes point to the same commit"
  [[ -f "$LOCALNET_ROOT/fixtures/$requested_chain/n.json" ]] || die "$requested_chain N fixture is missing"
  [[ -f "$LOCALNET_ROOT/fixtures/$requested_chain/n_minus_1.json" ]] || die "$requested_chain N-1 fixture is missing"
  [[ -f "$LOCALNET_ROOT/fixtures/$requested_chain/incompatible.json" ]] || die "$requested_chain incompatible fixture is missing"
  fixture_n_version=$(jq -r '.node_version // empty' "$LOCALNET_ROOT/fixtures/$requested_chain/n.json")
  fixture_n_minus_1_version=$(jq -r '.node_version // empty' "$LOCALNET_ROOT/fixtures/$requested_chain/n_minus_1.json")
  [[ "$fixture_n_version" == "$N_VERSION" ]] || die "$requested_chain N fixture version $fixture_n_version, expected $N_VERSION"
  [[ "$fixture_n_minus_1_version" == "$N_MINUS_1_VERSION" ]] || die "$requested_chain N-1 fixture version $fixture_n_minus_1_version, expected $N_MINUS_1_VERSION"
  if [[ "$requested_chain" == "cosmos" ]]; then
    fixture_n_reported=$(jq -r '.reported_version // empty' "$LOCALNET_ROOT/fixtures/$requested_chain/n.json")
    fixture_n_minus_1_reported=$(jq -r '.reported_version // empty' "$LOCALNET_ROOT/fixtures/$requested_chain/n_minus_1.json")
    [[ "$fixture_n_reported" == "$N_REPORTED_VERSION" ]] || die "cosmos N reported version mismatch"
    [[ "$fixture_n_minus_1_reported" == "$N_MINUS_1_REPORTED_VERSION" ]] || die "cosmos N-1 reported version mismatch"
  fi
done

# shellcheck source=/dev/null
source "$LOCALNET_ROOT/manifests/toxiproxy.env"
[[ "$TOXIPROXY_COMMIT" =~ ^[0-9a-f]{40}$ ]] || die "Toxiproxy commit is not a full SHA"
grep -Fq "$TOXIPROXY_IMAGE" "$LOCALNET_ROOT/compose.yaml" || die "compose.yaml does not use the locked Toxiproxy image"

printf 'all localnet manifests and fixture lanes are structurally valid\n'
