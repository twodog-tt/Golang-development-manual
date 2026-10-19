#!/usr/bin/env python3
"""Payment -> custody -> DeFi -> indexed evidence -> ledger -> merchant Inbox."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import secrets
import shlex
import shutil
import signal
import subprocess
import tempfile
import time
import uuid

OPS = Path(__file__).resolve().parents[1]
REPO = OPS.parents[1]
CHAIN = REPO / "examples/onchain-execution"
spec = importlib.util.spec_from_file_location("onchain_runner", CHAIN / "verify.py")
base = importlib.util.module_from_spec(spec)
spec.loader.exec_module(base)
run, capture, wait, port = base.run, base.capture, base.wait, base.port
IMAGES = ("postgres:17-alpine", "temporalio/temporal:1.5.1")


def verify(report, serve):
    if report:
        report.unlink(missing_ok=True)
    anvil = shutil.which("anvil")
    if not anvil or "1.4.4" not in capture([anvil, "--version"]).splitlines()[0]:
        raise RuntimeError("This lab requires Anvil 1.4.4")
    for image in IMAGES:
        run(["docker", "image", "inspect", image], stdout=subprocess.DEVNULL)
    env = os.environ.copy()
    env.pop("GOROOT", None)
    env["GOFLAGS"] = "-mod=readonly"
    go = os.environ.get("GO_BIN") or ("/opt/homebrew/bin/go" if Path("/opt/homebrew/bin/go").exists() else shutil.which("go"))
    for root in (OPS, CHAIN):
        run([go, "test", "-race", "./..."], cwd=root, env=env)
        run(["npm", "ci", "--ignore-scripts"], cwd=root / "client")
    run(["npm", "run", "build"], cwd=OPS / "client")
    run_id = "payment-e2e-" + uuid.uuid4().hex[:12]
    pg, temporal = run_id + "-pg", run_id + "-temporal"
    containers, processes = [], []
    with tempfile.TemporaryDirectory(prefix=run_id + "-") as tmp:
        out = Path(tmp)
        try:
            password = secrets.token_hex(24)
            for name, image, args in [
                (pg, IMAGES[0], ["-e", "POSTGRES_USER=lab", "-e", "POSTGRES_DB=lab", "-e", "POSTGRES_PASSWORD=" + password, "-p", "127.0.0.1::5432"]),
                (temporal, IMAGES[1], ["-p", "127.0.0.1::7233"]),
            ]:
                command = ["docker", "run", "-d", "--name", name, "--label", "knowledge-payment-e2e=" + run_id] + args + [image]
                if name == temporal:
                    command += ["server", "start-dev", "--ip", "0.0.0.0", "--db-filename", "/tmp/temporal.db"]
                capture(command)
                containers.append(name)
            wait(lambda: subprocess.run(["docker", "exec", pg, "pg_isready", "-h", "127.0.0.1", "-U", "lab"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0, "PostgreSQL TCP")
            wait(lambda: subprocess.run(["docker", "exec", temporal, "temporal", "operator", "namespace", "list"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0, "Temporal")
            pg_port = capture(["docker", "port", pg, "5432/tcp"]).split(":")[-1]
            temporal_port = capture(["docker", "port", temporal, "7233/tcp"]).split(":")[-1]
            api_port, gateway_port, anvil_port, provider_port = port(), port(), port(), port()
            env.update(DATABASE_URL=f"postgres://lab:{password}@127.0.0.1:{pg_port}/lab?sslmode=disable",
                       CHAIN_DATABASE_URL=f"postgres://lab:{password}@127.0.0.1:{pg_port}/chain?sslmode=disable",
                       TEMPORAL_ADDRESS="127.0.0.1:" + temporal_port,
                       API_URL=f"http://127.0.0.1:{api_port}", APP_ORIGIN=f"http://127.0.0.1:{api_port}",
                       CHAIN_GATEWAY_URL=f"http://127.0.0.1:{gateway_port}", CHANNEL_URL=f"http://127.0.0.1:{provider_port}",
                       ANVIL_URL=f"http://127.0.0.1:{anvil_port}", LAB_CHANNEL_SECRET=secrets.token_hex(32),
                       LAB_OUTPUT=tmp, LAB_SERVE="1" if serve else "", APPROVAL_SECONDS="300", OBSERVE_SECONDS="90",
                       LEDGER_ASSET="LABUSD", OUTPUT_LEDGER_ASSET="LABDAI")
            sql = (REPO / "examples/postgres-reliability/schema.sql").read_text() + (OPS / "internal/ops/schema.sql").read_text()
            sql = sql.replace("'USDC'", "'LABUSD'")
            sql += """
CREATE DATABASE chain;
INSERT INTO ops.wallets VALUES
 ('0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf','merchant_a','eoa'),
 ('0x2B5AD5c4795c026514f8317c7a215E218DcCD6cF','merchant_b','eoa');
INSERT INTO lab.accounts VALUES
 ('merchant_a','LABUSD',0,0),('merchant_b','LABUSD',0,0),
 ('merchant_a:out','LABDAI',0,0),('merchant_b:out','LABDAI',0,0),
 ('dex_input','LABUSD',0,0),('dex_output','LABDAI',1000000000000,1000000000000);
"""
            run(["docker", "exec", "-i", pg, "psql", "-Xq", "-v", "ON_ERROR_STOP=1", "-U", "lab", "-d", "lab"], input=sql, stdout=subprocess.DEVNULL)
            for name in ("api", "provider", "worker"):
                run([go, "build", "-o", str(out / name), "./cmd/" + name], cwd=OPS, env=env)
                env[name.upper() + "_BIN"] = str(out / name)
            run([go, "build", "-o", str(out / "gateway"), "./cmd/server"], cwd=CHAIN, env=env)
            env["GATEWAY_BIN"] = str(out / "gateway")
            with (out / "anvil.log").open("w") as log:
                processes.append(subprocess.Popen([anvil, "--host", "127.0.0.1", "--port", str(anvil_port), "--chain-id", "31337", "--hardfork", "cancun", "--silent"], stdout=log, stderr=subprocess.STDOUT))
            # The JS harness checks chain identity before deploying any contracts.
            integration = subprocess.Popen(["node", "payment-e2e.mjs"], cwd=CHAIN / "client", env=env, start_new_session=True)
            try:
                code = integration.wait(timeout=None if serve else 300)
                if code:
                    raise subprocess.CalledProcessError(code, "payment end-to-end integration")
            finally:
                try:
                    os.killpg(integration.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
                integration.wait()
            if serve:
                return
            result = json.loads((out / "payment-e2e.json").read_text())
            result.update(run_id=run_id, checked_at=time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
                          go=capture([go, "version"], env=env), node=capture(["node", "--version"]), anvil=capture([anvil, "--version"]),
                          images={image: capture(["docker", "image", "inspect", "--format", "{{.Id}}", image]) for image in IMAGES})
            result["source_sha256"] = {str(p.relative_to(REPO)): hashlib.sha256(p.read_bytes()).hexdigest()
                                       for root in (OPS, CHAIN) for p in sorted(root.rglob("*")) if p.is_file()
                                       and not {"node_modules", "dist", "__pycache__"}.intersection(p.relative_to(root).parts)
                                       and p.name not in ("local-verification.json", "local-e2e-verification.json")}
            result["source_sha256"]["examples/postgres-reliability/schema.sql"] = hashlib.sha256((REPO / "examples/postgres-reliability/schema.sql").read_bytes()).hexdigest()
            if report:
                report.parent.mkdir(parents=True, exist_ok=True)
                report.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n")
            print(json.dumps({"result": "passed", "checks": len(result["checks"]), "run_id": run_id}))
        except BaseException:
            if report:
                report.parent.mkdir(parents=True, exist_ok=True)
            for p in out.glob("*.log"):
                if report:
                    shutil.copy(p, report.parent / p.name)
                print(p.name + ":\n" + p.read_text()[-2500:])
            raise
        finally:
            for process in processes:
                process.terminate()
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()
            for name in reversed(containers):
                subprocess.run(["docker", "rm", "-fv", name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--report", type=Path)
    parser.add_argument("--serve", action="store_true")
    args = parser.parse_args()
    try:
        verify(args.report, args.serve)
    except KeyboardInterrupt:
        print("Local payment lab stopped")
