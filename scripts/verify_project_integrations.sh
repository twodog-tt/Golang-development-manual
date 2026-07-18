#!/usr/bin/env bash

set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Codex/Desktop shells can inherit a stale GOROOT even when a newer Homebrew
# Go binary is installed. Allow callers to override GO_BIN, prefer Homebrew on
# macOS when present, and always let the selected binary infer its own GOROOT.
if [[ -z "${GO_BIN:-}" ]]; then
  if [[ -x /opt/homebrew/bin/go ]]; then
    GO_BIN=/opt/homebrew/bin/go
  else
    GO_BIN="$(command -v go)"
  fi
fi

run_module() {
  local name="$1"
  local path="$2"
  shift 2

  echo "==> ${name}"
  (
    cd "${root}/${path}"
    "$@"
  )
}

run_module "durable signer + etcd/HSM/cross-process MPC" "examples/signer-project" env -u GOROOT "${GO_BIN}" test -mod=readonly -count=1 ./...
run_module "Solana endpoint adapter" "examples/non-evm-sdk/solana" env -u GOROOT "${GO_BIN}" test -mod=readonly -count=1 ./...
run_module "Cosmos endpoint adapter" "examples/non-evm-sdk/cosmos" env -u GOROOT GOMAXPROCS=2 "${GO_BIN}" test -mod=readonly -p=1 -count=1 ./...
run_module "Aptos endpoint adapter" "examples/non-evm-sdk/aptos" env -u GOROOT "${GO_BIN}" test -mod=readonly -count=1 ./...
run_module "Sui endpoint adapter" "examples/non-evm-sdk/sui" env -u GOROOT "${GO_BIN}" test -mod=readonly -count=1 ./...
run_module "four-chain manifests + offline localnet gates" "examples/non-evm-sdk/localnet" make manifests offline

echo "All project integrations passed deterministic tests and offline localnet gates."
