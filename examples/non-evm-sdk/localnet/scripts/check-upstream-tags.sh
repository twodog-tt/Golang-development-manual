#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=common.sh
source "$SCRIPT_DIR/common.sh"

[[ "${LOCALNET_ALLOW_NETWORK:-0}" == "1" ]] || die "upstream tag verification is networked and opt-in; set LOCALNET_ALLOW_NETWORK=1"
require_command git

verify_tag() {
  local repository=$1
  local tag=$2
  local expected_commit=$3
  local refs
  local actual_commit
  refs=$(git ls-remote --tags "$repository" "refs/tags/$tag" "refs/tags/$tag^{}")
  actual_commit=$(awk -v peeled="refs/tags/$tag^{}" '$2 == peeled {print $1}' <<<"$refs")
  if [[ -z "$actual_commit" ]]; then
    actual_commit=$(awk -v direct="refs/tags/$tag" '$2 == direct {print $1}' <<<"$refs")
  fi
  [[ -n "$actual_commit" ]] || die "upstream tag $repository $tag was not found"
  [[ "$actual_commit" == "$expected_commit" ]] || die "upstream tag $repository $tag resolves to $actual_commit; expected source commit $expected_commit"
}

for requested_chain in solana cosmos aptos sui; do
  load_chain_manifest "$requested_chain"
  verify_tag "$SOURCE_REPOSITORY" "$N_TAG" "$N_COMMIT"
  verify_tag "$SOURCE_REPOSITORY" "$N_MINUS_1_TAG" "$N_MINUS_1_COMMIT"
  printf '%s upstream tags still match locked commits\n' "$requested_chain"
done

# shellcheck source=/dev/null
source "$LOCALNET_ROOT/manifests/toxiproxy.env"
verify_tag "$TOXIPROXY_SOURCE_REPOSITORY" "$TOXIPROXY_TAG" "$TOXIPROXY_COMMIT"
printf 'Toxiproxy upstream tag still matches the locked commit\n'
