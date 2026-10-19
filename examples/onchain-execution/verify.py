#!/usr/bin/env python3
"""Run isolated Anvil + PostgreSQL + Go + real Uniswap V2 fault verification."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import secrets
import shutil
import signal
import socket
import subprocess
import tempfile
import time
import urllib.request
import uuid

ROOT = Path(__file__).resolve().parent
IMAGE = "postgres:17-alpine"


def run(args, **kwargs):
    return subprocess.run(args, check=True, text=True, **kwargs)


def capture(args, **kwargs):
    return run(args, stdout=subprocess.PIPE, **kwargs).stdout.strip()


def port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def wait(check, name):
    deadline = time.monotonic() + 60
    while time.monotonic() < deadline:
        try:
            if check():
                return
        except (OSError, subprocess.CalledProcessError):
            pass
        time.sleep(0.2)
    raise RuntimeError("Timed out waiting for " + name)


def verify(report, serve):
    if report:
        report.unlink(missing_ok=True)
    anvil = shutil.which("anvil")
    if not anvil:
        raise RuntimeError("Install Foundry v1.4.4 (Anvil) first; see README")
    version = capture([anvil, "--version"])
    if "1.4.4" not in version.splitlines()[0]:
        raise RuntimeError("This lab pins Anvil 1.4.4; actual: " + version.splitlines()[0])
    run(["docker", "image", "inspect", IMAGE], stdout=subprocess.DEVNULL)
    env = os.environ.copy()
    env.pop("GOROOT", None)
    env["GOFLAGS"] = "-mod=readonly"
    go = os.environ.get("GO_BIN") or ("/opt/homebrew/bin/go" if Path("/opt/homebrew/bin/go").exists() else shutil.which("go"))
    run([go, "test", "-race", "./..."], cwd=ROOT, env=env)
    run(["npm", "ci", "--ignore-scripts"], cwd=ROOT / "client")
    run_id = "onchain-execution-" + uuid.uuid4().hex[:12]
    name = run_id + "-pg"
    started = False
    processes = []
    with tempfile.TemporaryDirectory(prefix=run_id + "-") as tmp:
        out = Path(tmp)
        try:
            password = secrets.token_hex(24)
            capture(["docker", "run", "-d", "--name", name, "--label", "knowledge-onchain-lab=" + run_id,
                     "-e", "POSTGRES_USER=lab", "-e", "POSTGRES_DB=lab", "-e", "POSTGRES_PASSWORD=" + password,
                     "-p", "127.0.0.1::5432", IMAGE])
            started = True
            wait(lambda: subprocess.run(["docker", "exec", name, "pg_isready", "-h", "127.0.0.1", "-U", "lab"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0, "PostgreSQL TCP")
            pg_port = capture(["docker", "port", name, "5432/tcp"]).split(":")[-1]
            anvil_port, api_port = port(), port()
            env.update(DATABASE_URL=f"postgres://lab:{password}@127.0.0.1:{pg_port}/lab?sslmode=disable",
                       ANVIL_URL=f"http://127.0.0.1:{anvil_port}", API_URL=f"http://127.0.0.1:{api_port}",
                       LAB_OUTPUT=tmp, SERVER_BIN=str(out / "server"), LAB_SERVE="1" if serve else "")
            run([go, "build", "-o", str(out / "server"), "./cmd/server"], cwd=ROOT, env=env)
            with (out / "anvil.log").open("w") as log:
                processes.append(subprocess.Popen([anvil, "--host", "127.0.0.1", "--port", str(anvil_port), "--chain-id", "31337", "--hardfork", "cancun", "--silent"], stdout=log, stderr=subprocess.STDOUT))

            def ready():
                body = json.dumps({"jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []}).encode()
                req = urllib.request.Request(env["ANVIL_URL"], data=body, headers={"Content-Type": "application/json"})
                with urllib.request.urlopen(req, timeout=1) as response:
                    return json.load(response).get("result") == "0x7a69"

            wait(ready, "Anvil")
            integration = subprocess.Popen(["node", "integration.mjs"], cwd=ROOT / "client", env=env, start_new_session=True)
            try:
                code = integration.wait(timeout=None if serve else 240)
                if code:
                    raise subprocess.CalledProcessError(code, "onchain integration")
            finally:
                try:
                    os.killpg(integration.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
                integration.wait()
            result = json.loads((out / "integration.json").read_text())
            result.update(run_id=run_id, checked_at=time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
                          anvil=version, go=capture([go, "version"], env=env), node=capture(["node", "--version"]),
                          image={IMAGE: capture(["docker", "image", "inspect", "--format", "{{.Id}}", IMAGE])})
            result["source_sha256"] = {str(p.relative_to(ROOT)): hashlib.sha256(p.read_bytes()).hexdigest()
                                       for p in sorted(ROOT.rglob("*")) if p.is_file()
                                       and not {"node_modules", "__pycache__"}.intersection(p.relative_to(ROOT).parts)
                                       and p.name != "local-verification.json"}
            if report:
                report.parent.mkdir(parents=True, exist_ok=True)
                report.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n")
            print(json.dumps({"result": "passed", "checks": len(result["checks"]), "run_id": run_id}))
        except BaseException:
            for filename in ("gateway.log", "anvil.log"):
                p = out / filename
                if p.exists():
                    print(filename + ":\n" + p.read_text()[-5000:])
            raise
        finally:
            for process in processes:
                process.terminate()
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()
            if started:
                subprocess.run(["docker", "rm", "-fv", name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--report", type=Path)
    parser.add_argument("--serve", action="store_true", help="keep the verified local fixture running for curl exploration")
    args = parser.parse_args()
    try:
        verify(args.report, args.serve)
    except KeyboardInterrupt:
        print("Local lab stopped")
