#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"
if [[ -z "${GO_BIN:-}" ]]; then
  if [[ -x /opt/homebrew/bin/go ]]; then
    GO_BIN=/opt/homebrew/bin/go
  else
    GO_BIN="$(command -v go)"
  fi
fi
if [[ $# -gt 1 || ( $# -eq 1 && "$1" != "--offline" ) ]]; then
  echo "Usage: $0 [--offline]" >&2
  exit 2
fi
task_tmp="$(mktemp -d)"
trap 'rm -rf "$task_tmp"' EXIT
env -u GOROOT "$GO_BIN" test -race ./examples/senior/agenteval/...
env -u GOROOT "$GO_BIN" build -o "$task_tmp/eval" ./examples/senior/agenteval/cmd/eval
"$task_tmp/eval" > "$task_tmp/good.json"
python3 - "$task_tmp" <<'PY'
import json
from pathlib import Path
import sys
candidate = json.loads(Path('examples/senior/agenteval/testdata/candidate-good.json').read_text())
del candidate['runs'][0]['cost_microusd']
(Path(sys.argv[1]) / 'incomplete.json').write_text(json.dumps(candidate))
PY
status=0
"$task_tmp/eval" -candidate "$task_tmp/incomplete.json" > "$task_tmp/incomplete-result.json" 2> "$task_tmp/incomplete-error.txt" || status=$?
if [[ "$status" -ne 2 ]]; then
  echo "Expected incomplete trace rejection (exit 2), got $status" >&2
  exit 1
fi
status=0
"$task_tmp/eval" -candidate examples/senior/agenteval/testdata/candidate-unsafe.json > "$task_tmp/unsafe.json" || status=$?
if [[ "$status" -ne 1 ]]; then
  echo "Expected release rejection (exit 1), got $status" >&2
  exit 1
fi
python3 - "$task_tmp" <<'PY'
import json
from pathlib import Path
import sys
p = Path(sys.argv[1])
good = json.loads((p / 'good.json').read_text())
bad = json.loads((p / 'unsafe.json').read_text())
if not good['gate_passed'] or bad['gate_passed'] or bad['critical_failures'] != 1:
    raise SystemExit('Agent release gate did not enforce the safety failure')
print('PASS: Agent positive fixture accepted; unsafe fixture rejected with exit 1; incomplete trace rejected with exit 2')
PY
if [[ "${1:-}" != "--offline" ]]; then
  python3 examples/postgres-reliability/verify.py
fi
