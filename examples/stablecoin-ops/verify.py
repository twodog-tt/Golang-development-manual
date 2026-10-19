#!/usr/bin/env python3
"""Run isolated PostgreSQL + Temporal services and the cross-language fault lab."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import secrets
import shlex
import signal
import shutil
import socket
import subprocess
import tempfile
import time
import urllib.request
import uuid

ROOT = Path(__file__).resolve().parent
REPO = ROOT.parents[1]
PG_IMAGE = os.environ.get("LAB_PG_IMAGE", "postgres:17-alpine")
TEMPORAL_IMAGE = os.environ.get("LAB_TEMPORAL_IMAGE", "temporalio/temporal:1.5.1")


def run(args, **kwargs):
    return subprocess.run(args, check=True, text=True, **kwargs)


def capture(args, **kwargs):
    return run(args, stdout=subprocess.PIPE, **kwargs).stdout.strip()


def port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def wait(check, name, seconds=90):
    deadline = time.monotonic() + seconds
    while time.monotonic() < deadline:
        try:
            if check():
                return
        except (OSError, subprocess.CalledProcessError):
            pass
        time.sleep(0.25)
    raise RuntimeError("Timed out waiting for " + name)


def verify(report, serve=False):
    for image in (PG_IMAGE, TEMPORAL_IMAGE):
        run(["docker", "image", "inspect", image], stdout=subprocess.DEVNULL)
    run_id = "stablecoin-ops-" + uuid.uuid4().hex[:12]
    pg, temporal = run_id + "-pg", run_id + "-temporal"
    containers, processes, logs = [], [], []
    env = os.environ.copy()
    env.pop("GOROOT", None)
    env["GOFLAGS"] = "-mod=mod"
    go = os.environ.get("GO_BIN") or ("/opt/homebrew/bin/go" if Path("/opt/homebrew/bin/go").exists() else shutil.which("go"))
    with tempfile.TemporaryDirectory(prefix=run_id + "-") as tmp:
        out = Path(tmp)
        try:
            # No existing databases, mounts or cloud credentials are used.
            for name, image, args in [
                (pg, PG_IMAGE, ["-e", "POSTGRES_USER=lab", "-e", "POSTGRES_DB=lab", "-e", "POSTGRES_PASSWORD=" + secrets.token_hex(16), "-p", "127.0.0.1::5432"]),
                (temporal, TEMPORAL_IMAGE, ["-p", "127.0.0.1::7233"]),
            ]:
                command = ["docker", "run", "-d", "--name", name, "--label", "knowledge-stablecoin-lab=" + run_id] + args + [image]
                if name == temporal:
                    command += ["server", "start-dev", "--ip", "0.0.0.0", "--db-filename", "/tmp/temporal.db"]
                capture(command)
                containers.append(name)
            # The image's temporary init server accepts Unix sockets before it
            # shuts down. Only the final server accepts TCP connections.
            wait(lambda: subprocess.run(["docker", "exec", pg, "pg_isready", "-h", "127.0.0.1", "-U", "lab"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0, "PostgreSQL TCP")
            wait(lambda: subprocess.run(["docker", "exec", temporal, "temporal", "operator", "namespace", "list"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0, "Temporal")
            pg_port = capture(["docker", "port", pg, "5432/tcp"]).split(":")[-1]
            temporal_port = capture(["docker", "port", temporal, "7233/tcp"]).split(":")[-1]
            # Read the generated local-fixture password without logging it.
            config = json.loads(capture(["docker", "inspect", pg]))[0]
            password = next(v.split("=", 1)[1] for v in config["Config"]["Env"] if v.startswith("POSTGRES_PASSWORD="))
            env.update(DATABASE_URL=f"postgres://lab:{password}@127.0.0.1:{pg_port}/lab?sslmode=disable", TEMPORAL_ADDRESS="127.0.0.1:" + temporal_port,
                       LAB_CHANNEL_SECRET=secrets.token_hex(32), LAB_PG_CONTAINER=pg, LAB_OUTPUT=tmp,
                       LAB_AUTH_INTEGRATION="1", APPROVAL_SECONDS="300" if serve else "8", OBSERVE_SECONDS="300" if serve else "8")
            api_port, provider_port = port(), port()
            env.update(API_URL=f"http://127.0.0.1:{api_port}", APP_ORIGIN=f"http://127.0.0.1:{api_port}", CHANNEL_URL=f"http://127.0.0.1:{provider_port}")
            sql = (REPO / "examples/postgres-reliability/schema.sql").read_text() + (ROOT / "internal/ops/schema.sql").read_text()
            sql += """
INSERT INTO ops.wallets VALUES
('0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf','merchant_a','eoa'),
('0x2B5AD5c4795c026514f8317c7a215E218DcCD6cF','merchant_b','eoa');
INSERT INTO lab.accounts VALUES ('merchant_a','USDC',0,0),('merchant_b','USDC',0,0);
"""
            run(["docker", "exec", "-i", pg, "psql", "-Xq", "-v", "ON_ERROR_STOP=1", "-U", "lab", "-d", "lab"], input=sql, stdout=subprocess.DEVNULL)
            run([go, "test", "-race", "./..."], cwd=ROOT, env=env)
            for name in ("api", "provider", "worker", "history"):
                run([go, "build", "-o", str(out / name), "./cmd/" + name], cwd=ROOT, env=env)
            env.update(WORKER_BIN=str(out / "worker"), HISTORY_BIN=str(out / "history"))
            run(["npm", "ci", "--ignore-scripts"], cwd=ROOT / "client")
            run(["npm", "run", "build"], cwd=ROOT / "client")
            for name, listen in (("provider", provider_port), ("api", api_port)):
                log = (out / (name + ".log")).open("w")
                logs.append(log)
                child = subprocess.Popen([str(out / name)], env={**env, "LISTEN_ADDR": "127.0.0.1:" + str(listen)}, stdout=log, stderr=subprocess.STDOUT)
                processes.append(child)
            def ready():
                with urllib.request.urlopen(env["API_URL"] + "/health", timeout=1) as response:
                    return response.status == 200
            wait(ready, "Go API")
            if serve:
                log = (out / "worker.log").open("w")
                logs.append(log)
                processes.append(subprocess.Popen([str(out / "worker")], env=env, stdout=log, stderr=subprocess.STDOUT))
                session = out / "lab.env"
                session.write_text("\n".join("export " + key + "=" + shlex.quote(env[key]) for key in ("DATABASE_URL", "TEMPORAL_ADDRESS", "API_URL", "CHANNEL_URL", "LAB_CHANNEL_SECRET", "LAB_PG_CONTAINER")) + "\n")
                session.chmod(0o600)
                print("Local fixture ready. In another terminal: source " + shlex.quote(str(session)), flush=True)
                print("Then cd examples/stablecoin-ops/client && npm run ops -- ops", flush=True)
                input("Press Enter or Ctrl-C here to stop and remove this run's containers: ")
                return
            # A separate process group also owns the Worker children created by Node.
            # On timeout/interrupt, kill that group before removing its databases.
            integration = subprocess.Popen(["node", "dist/integration.js"], cwd=ROOT / "client", env=env, start_new_session=True)
            try:
                code = integration.wait(timeout=180)
                if code:
                    raise subprocess.CalledProcessError(code, "stablecoin TypeScript integration")
            finally:
                try:
                    os.killpg(integration.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
                integration.wait()
            result = json.loads((out / "integration.json").read_text())
            result.update(run_id=run_id, checked_at=time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), go=capture([go, "version"], env=env), node=capture(["node", "--version"]),
                          images={image: capture(["docker", "image", "inspect", "--format", "{{.Id}}", image]) for image in (PG_IMAGE, TEMPORAL_IMAGE)},
                          scope="Local PostgreSQL/Temporal, real Go and viem client; mock channel and ERC-1271 RPC fixture; no real tokens or external PSP")
            result["source_sha256"] = {str(p.relative_to(REPO)): hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(ROOT.rglob("*"))
                                       if p.is_file() and not {"node_modules", "dist", "__pycache__"}.intersection(p.relative_to(ROOT).parts) and p.name != "local-verification.json"}
            # Preserve the actual replay history alongside the report when requested.
            if report:
                report.parent.mkdir(parents=True, exist_ok=True)
                report.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n")
                shutil.copy(out / "history.json", report.with_suffix(".history.json"))
            print(json.dumps({"result": "passed", "checks": len(result["checks"]), "run_id": run_id}))
        except BaseException:
            for log in logs:
                log.flush()
            for name in ("api", "provider", "worker"):
                path = out / (name + ".log")
                if path.exists():
                    print(name + " log:\n" + path.read_text()[-4000:])
            raise
        finally:
            for child in processes:
                child.terminate()
            for child in processes:
                try:
                    child.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    child.kill()
                    child.wait()
            for log in logs:
                log.close()
            for name in reversed(containers):
                subprocess.run(["docker", "rm", "-fv", name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--report", type=Path)
    parser.add_argument("--serve", action="store_true", help="keep the isolated local fixture available for CLI exploration")
    args = parser.parse_args()
    try:
        verify(args.report, args.serve)
    except KeyboardInterrupt:
        print("Local lab stopped")
