#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=common.sh
source "$SCRIPT_DIR/common.sh"

requested_chain=${1:-}
requested_lane=${2:-}
validate_chain "$requested_chain"
validate_lane "$requested_lane"
[[ "${LOCALNET_ALLOW_NETWORK:-0}" == "1" ]] || die "building pinned sources is networked and opt-in; set LOCALNET_ALLOW_NETWORK=1"

require_command git
load_chain_manifest "$requested_chain"
select_lane "$requested_lane"

source_dir="$CACHE_ROOT/src/$requested_chain/$requested_lane"
mkdir -p "$(dirname "$source_dir")" "$(dirname "$SELECTED_BINARY")"

if [[ ! -d "$source_dir/.git" ]]; then
  git clone --filter=blob:none --no-checkout "$SOURCE_REPOSITORY" "$source_dir"
fi
actual_remote=$(git -C "$source_dir" remote get-url origin)
[[ "$actual_remote" == "$SOURCE_REPOSITORY" ]] || die "source cache origin $actual_remote, expected $SOURCE_REPOSITORY"
git -C "$source_dir" fetch --depth=1 origin "$SELECTED_TAG"
git -C "$source_dir" checkout --detach "$SELECTED_COMMIT"
actual_commit=$(git -C "$source_dir" rev-parse HEAD)
[[ "$actual_commit" == "$SELECTED_COMMIT" ]] || die "checked out $actual_commit, expected $SELECTED_COMMIT"

jobs=${LOCALNET_BUILD_JOBS:-2}
case "$requested_chain" in
  solana)
    require_command cargo
    cargo build --manifest-path "$source_dir/Cargo.toml" --locked --release --jobs "$jobs" --bin solana-test-validator
    install -m 0755 "$source_dir/target/release/solana-test-validator" "$SELECTED_BINARY"
    ;;
  cosmos)
    (
      cd "$source_dir"
      GOWORK=off run_go build -mod=readonly -trimpath -o "$SELECTED_BINARY" ./cmd/cometbft
    )
    ;;
  aptos)
    require_command cargo
    cargo build --manifest-path "$source_dir/Cargo.toml" --locked --release --jobs "$jobs" -p aptos --bin aptos
    install -m 0755 "$source_dir/target/release/aptos" "$SELECTED_BINARY"
    ;;
  sui)
    require_command cargo
    cargo build --manifest-path "$source_dir/Cargo.toml" --locked --release --jobs "$jobs" -p sui --bin sui
    install -m 0755 "$source_dir/target/release/sui" "$SELECTED_BINARY"
    ;;
esac

{
  printf 'chain=%s\n' "$requested_chain"
  printf 'lane=%s\n' "$requested_lane"
  printf 'tag=%s\n' "$SELECTED_TAG"
  printf 'commit=%s\n' "$SELECTED_COMMIT"
  printf 'sha256=%s\n' "$(sha256_file "$SELECTED_BINARY")"
} > "$SELECTED_BINARY.provenance"

verify_binary "$requested_chain" "$requested_lane"
printf 'built pinned %s %s at %s\n' "$requested_chain" "$requested_lane" "$SELECTED_BINARY"
