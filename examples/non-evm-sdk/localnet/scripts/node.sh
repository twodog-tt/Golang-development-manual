#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=common.sh
source "$SCRIPT_DIR/common.sh"

action=${1:-}
requested_chain=${2:-}
validate_chain "$requested_chain"
load_chain_manifest "$requested_chain"

state_dir=$(chain_state_dir "$requested_chain")
data_dir="$state_dir/data"
pid_file="$state_dir/node.pid"
binary_file="$state_dir/node.binary"
lane_file="$state_dir/lane"
log_file="$state_dir/node.log"

read_pid() {
  [[ -f "$pid_file" ]] || return 1
  tr -d '\r\n' < "$pid_file"
}

is_running() {
  local pid
  local expected_binary
  local process_args
  pid=$(read_pid) || return 1
  kill -0 "$pid" 2>/dev/null || return 1
  [[ -f "$binary_file" ]] || return 1
  expected_binary=$(tr -d '\r\n' < "$binary_file")
  [[ -n "$expected_binary" ]] || return 1
  process_args=$(ps -p "$pid" -o args= 2>/dev/null) || return 1
  [[ "$process_args" == *"$expected_binary"* ]]
}

stop_node() {
  local pid
  local expected_binary
  local process_args
  if ! pid=$(read_pid); then
    return
  fi
  if kill -0 "$pid" 2>/dev/null; then
    [[ -f "$binary_file" ]] || die "refusing to signal PID $pid without a recorded node binary"
    expected_binary=$(tr -d '\r\n' < "$binary_file")
    process_args=$(ps -p "$pid" -o args= 2>/dev/null || true)
    [[ -n "$expected_binary" && "$process_args" == *"$expected_binary"* ]] ||
      die "refusing to signal stale PID $pid; process does not match the recorded node binary"
    kill -TERM "$pid" 2>/dev/null || true
    for _ in $(seq 1 30); do
      kill -0 "$pid" 2>/dev/null || break
      sleep 1
    done
    if kill -0 "$pid" 2>/dev/null; then
      kill -KILL "$pid" 2>/dev/null || true
    fi
  fi
  rm -f "$pid_file" "$binary_file"
}

start_node() {
  local requested_lane=$1
  local state_mode=$2
  validate_lane "$requested_lane"
  case "$state_mode" in
    fresh|reuse) ;;
    *) die "state mode must be fresh or reuse" ;;
  esac
  is_running && die "$requested_chain node is already running"

  verify_binary "$requested_chain" "$requested_lane"
  load_chain_manifest "$requested_chain"
  select_lane "$requested_lane"

  mkdir -p "$state_dir"
  if [[ "$state_mode" == "fresh" ]]; then
    rm -rf "$data_dir"
    rm -f "$(identity_file "$requested_chain")"
  fi
  mkdir -p "$data_dir"

  local -a command
  case "$requested_chain" in
    solana)
      command=("$SELECTED_BINARY" --ledger "$data_dir/ledger" --rpc-port 8899)
      if [[ "$state_mode" == "fresh" ]]; then
        command+=(--reset)
      fi
      ;;
    cosmos)
      require_command jq
      if [[ ! -f "$data_dir/config/genesis.json" ]]; then
        "$SELECTED_BINARY" init --home "$data_dir" >/dev/null
        jq --arg chain_id "$EXPECTED_IDENTITY" '.chain_id = $chain_id' \
          "$data_dir/config/genesis.json" > "$data_dir/config/genesis.json.tmp"
        mv "$data_dir/config/genesis.json.tmp" "$data_dir/config/genesis.json"
      fi
      command=(
        "$SELECTED_BINARY" node
        --home "$data_dir"
        --proxy_app=kvstore
        --rpc.laddr=tcp://127.0.0.1:26657
        --p2p.laddr=tcp://127.0.0.1:26656
      )
      ;;
    aptos)
      command=(
        "$SELECTED_BINARY" node run-localnet
        --test-dir "$data_dir"
        --assume-yes
        --no-faucet
        --no-txn-stream
        --ready-server-listen-port 8070
      )
      if [[ "$state_mode" == "fresh" ]]; then
        command+=(--force-restart)
      fi
      ;;
    sui)
      mkdir -p "$state_dir/tmp"
      if [[ "$state_mode" == "fresh" ]]; then
        # The pinned Sui CLI rejects --force-regenesis together with
        # --network.config. Generate a persistent config first so the same
        # directory can be reopened by the N binary during the upgrade gate.
        "$SELECTED_BINARY" genesis \
          --working-dir "$data_dir/config" \
          --force \
          --committee-size 1 \
          >"$state_dir/genesis.log" 2>&1
      fi
      command=(
        env
        TMPDIR="$state_dir/tmp"
        RUST_LOG=off,sui_node=info
        "$SELECTED_BINARY" start
        --network.config "$data_dir/config"
        --with-graphql=127.0.0.1:9125
        --fullnode-rpc-port 9000
      )
      ;;
  esac

  printf 'starting %s lane=%s version=%s state=%s\n' "$requested_chain" "$requested_lane" "$SELECTED_VERSION" "$state_mode"
  nohup "${command[@]}" >"$log_file" 2>&1 &
  local pid=$!
  printf '%s\n' "$SELECTED_BINARY" > "$binary_file"
  printf '%s\n' "$pid" > "$pid_file"
  printf '%s\n' "$requested_lane" > "$lane_file"

  if ! LOCALNET_WATCH_PID="$pid" "$SCRIPT_DIR/probe.sh" wait "$requested_chain" "$DIRECT_ENDPOINT" none; then
    tail -n 80 "$log_file" >&2 || true
    stop_node
    die "$requested_chain failed its readiness gate"
  fi
}

case "$action" in
  start)
    start_node "${3:-}" "${4:-fresh}"
    ;;
  stop)
    stop_node
    ;;
  restart)
    [[ -f "$lane_file" ]] || die "cannot restart $requested_chain without a recorded lane"
    recorded_lane=$(tr -d '\r\n' < "$lane_file")
    stop_node
    start_node "$recorded_lane" reuse
    ;;
  status)
    if is_running; then
      printf '%s is running with pid %s\n' "$requested_chain" "$(read_pid)"
    else
      printf '%s is stopped\n' "$requested_chain"
      exit 1
    fi
    ;;
  version)
    verify_binary "$requested_chain" "${3:-}"
    ;;
  *)
    die "usage: node.sh <start|stop|restart|status|version> <chain> [lane] [fresh|reuse]"
    ;;
esac
