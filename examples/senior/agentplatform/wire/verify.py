"""Build Go once; run Go/Python/TypeScript against committed positive/negative vectors."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[3]


def main():
    env = dict(os.environ)
    env.pop("GOROOT", None)
    go = env.get("GO_BIN") or ("/opt/homebrew/bin/go" if Path("/opt/homebrew/bin/go").exists() else "go")
    vectors = json.loads((HERE.parent / "testdata/wire.json").read_text())
    with tempfile.TemporaryDirectory(prefix="agentplatform-wire-") as temporary:
        binary = str(Path(temporary) / "canonical")
        subprocess.run([go, "build", "-o", binary, "./examples/senior/agentplatform/cmd/canonical"], cwd=ROOT, env=env, check=True)
        commands = [[binary], [sys.executable, str(HERE / "canonical.py")],
                    ["node", "--experimental-strip-types", "--disable-warning=ExperimentalWarning", str(HERE / "canonical.ts")]]
        for vector in vectors:
            raw = bytes.fromhex(vector["hex"]) if "hex" in vector else vector["raw"].encode()
            for command in commands:
                result = subprocess.run(command, input=raw, capture_output=True, timeout=10)
                if vector.get("reject"):
                    if result.returncode != 2:
                        raise RuntimeError((vector["name"], command, result.stdout, result.stderr))
                else:
                    if result.returncode != 0:
                        raise RuntimeError((vector["name"], command, result.stderr))
                    if json.loads(result.stdout) != vector["expected"]:
                        raise RuntimeError((vector["name"], command, result.stdout))
        print(f"PASS: {len(vectors)} wire vectors x Go/Python/TypeScript (Node type stripping)")


if __name__ == "__main__":
    main()
