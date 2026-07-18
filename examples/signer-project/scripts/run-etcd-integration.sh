#!/usr/bin/env bash

set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ -z "${ETCD_BIN:-}" ]]; then
  ETCD_BIN="$(command -v etcd || true)"
fi
if [[ -z "${ETCDCTL_BIN:-}" ]]; then
  ETCDCTL_BIN="$(command -v etcdctl || true)"
fi
if [[ -z "${GO_BIN:-}" ]]; then
  if [[ -x /opt/homebrew/bin/go ]]; then
    GO_BIN=/opt/homebrew/bin/go
  else
    GO_BIN="$(command -v go || true)"
  fi
fi
for required in ETCD_BIN ETCDCTL_BIN GO_BIN; do
  if [[ -z "${!required}" || ! -x "${!required}" ]]; then
    echo "${required} must name an executable" >&2
    exit 2
  fi
done

ETCD_EXPECTED_VERSION="${ETCD_EXPECTED_VERSION:-3.6.12}"
actual_version="$("$ETCD_BIN" --version | awk -F': ' '/^etcd Version:/ {print $2; exit}')"
if [[ -n "$ETCD_EXPECTED_VERSION" && "$actual_version" != "$ETCD_EXPECTED_VERSION" ]]; then
  echo "etcd version is ${actual_version:-unknown}, want $ETCD_EXPECTED_VERSION" >&2
  exit 2
fi

work="$(mktemp -d "${TMPDIR:-/tmp}/signer-etcd.XXXXXX")"
pids=()

cleanup() {
  local status=$?
  for pid in "${pids[@]:-}"; do
    kill "$pid" >/dev/null 2>&1 || true
  done
  for pid in "${pids[@]:-}"; do
    wait "$pid" >/dev/null 2>&1 || true
  done
  if (( status != 0 )); then
    for log in "$work"/*.log; do
      [[ -e "$log" ]] || continue
      echo "==> $log" >&2
      tail -200 "$log" >&2 || true
    done
  fi
  rm -rf "$work"
  exit "$status"
}
trap cleanup EXIT INT TERM

cluster="signer-etcd-1=http://127.0.0.1:12380,signer-etcd-2=http://127.0.0.1:22380,signer-etcd-3=http://127.0.0.1:32380"
endpoints="http://127.0.0.1:12379,http://127.0.0.1:22379,http://127.0.0.1:32379"

start_member() {
  local name="$1"
  local client_port="$2"
  local peer_port="$3"
  "$ETCD_BIN" \
    --name "$name" \
    --data-dir "$work/$name" \
    --listen-client-urls "http://127.0.0.1:$client_port" \
    --advertise-client-urls "http://127.0.0.1:$client_port" \
    --listen-peer-urls "http://127.0.0.1:$peer_port" \
    --initial-advertise-peer-urls "http://127.0.0.1:$peer_port" \
    --initial-cluster "$cluster" \
    --initial-cluster-state new \
    --initial-cluster-token signer-fence-integration-v1 \
    --logger zap \
    --log-level warn \
    >"$work/$name.log" 2>&1 &
  pids+=("$!")
}

start_member signer-etcd-1 12379 12380
start_member signer-etcd-2 22379 22380
start_member signer-etcd-3 32379 32380

healthy=0
for _ in {1..60}; do
  if ETCDCTL_API=3 "$ETCDCTL_BIN" \
    --endpoints="$endpoints" endpoint health --cluster >/dev/null 2>&1; then
    healthy=1
    break
  fi
  sleep 0.5
done
if (( healthy == 0 )); then
  echo "three-member etcd cluster did not become healthy" >&2
  exit 1
fi

ETCD_ENDPOINTS="$endpoints" \
ETCD_REQUIRE_THREE_MEMBERS=1 \
env -u GOROOT "$GO_BIN" test -count=1 \
  -tags=etcd_integration \
  -run '^TestEtcd' \
  ./fence
