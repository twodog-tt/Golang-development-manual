#!/usr/bin/env python3
"""Run built fixture CLIs; no network beyond loopback and no external credentials."""
import argparse
import json
import os
from pathlib import Path
import select
import subprocess
import tempfile


def check(condition, message):
    if not condition:
        raise RuntimeError(message)


def stdio(binary):
    env = dict(os.environ, MCP_DEMO_TOKEN="local-stdio-fixture")
    p = subprocess.Popen([binary], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                         stderr=subprocess.PIPE, text=True, bufsize=1, env=env)
    def send(value):
        p.stdin.write(json.dumps(value) + "\n")
        p.stdin.flush()
    def call(number, method, params):
        send(dict(jsonrpc="2.0", id=number, method=method, params=params))
        while True:
            check(select.select([p.stdout], [], [], 5)[0], "stdio response timed out")
            line = p.stdout.readline()
            check(line, "stdio server exited early")
            result = json.loads(line)  # also rejects stdout logging corruption
            if result.get("id") == number:
                return result
    try:
        init = call(1, "initialize", dict(protocolVersion="2025-11-25", capabilities={},
                    clientInfo=dict(name="offline-verifier", version="1")))
        check(init["result"]["protocolVersion"] == "2025-11-25", "version mismatch")
        send(dict(jsonrpc="2.0", method="notifications/initialized"))
        tools = call(2, "tools/list", {})["result"]["tools"]
        check(len(tools) == 2 and all(t.get("outputSchema") for t in tools), "schema missing")
        result = call(3, "tools/call", dict(name="get_order", arguments=dict(order_id="10001")))["result"]
        check(result["structuredContent"]["amount_minor"] == 9900, "wrong structured order")
        check(json.loads(result["content"][0]["text"]) == result["structuredContent"], "output mismatch")
        denied = call(4, "tools/call", dict(name="get_order", arguments=dict(order_id="10002")))["result"]
        check(denied["isError"], "cross-tenant order was accepted")
        invalid = call(5, "tools/call", dict(name="get_order", arguments=dict(order_id="10001", tenant_id="tenant-b")))
        check(invalid["error"]["code"] == -32602, "model tenant override accepted")
    finally:
        p.stdin.close()
        try:
            p.wait(timeout=5)
        except subprocess.TimeoutExpired:
            p.kill()
            p.wait()
        p.stdout.close()
        p.stderr.close()
    check(p.returncode == 0, "stdio server did not shut down cleanly")
    print("PASS: real stdio process, explicit 2025-11-25, schemas, outputs, tenant denial, pure stdout")


def a2a(binary):
    with tempfile.TemporaryDirectory(prefix="mcp-a2a-cli-") as root:
        def step(name, action, *args, expected=0):
            p = subprocess.run([binary, "-dir", str(Path(root) / name), "-step", action, *args],
                               capture_output=True, text=True, timeout=10)
            check(p.returncode == expected, f"{action} expected exit {expected}, got {p.returncode}: {p.stderr}")
            return json.loads(p.stdout) if p.stdout else None
        normal = step("normal", "demo")
        check(normal["state"] == "TASK_STATE_COMPLETED" and normal["effects"] == 1, "normal demo failed")
        check(step("normal", "demo")["effects"] == 1, "demo replay duplicated an effect")
        unknown = step("lost", "submit", "-lose-reply", expected=2)
        check(unknown["state"] == "UNKNOWN" and not unknown["task_id"], "first-submit loss not quarantined")
        step("lost", "submit", expected=2)
        recovered = step("lost", "reconcile")
        check(recovered["state"] == "TASK_STATE_AUTH_REQUIRED", "message dedup recovery failed")
        # CLI requires an actual reviewed digest/revision instead of approving the current object implicitly.
        step("lost", "approve", expected=2)
        step("lost", "approve", "-digest", recovered["review_digest"], "-revision", str(recovered["review_revision"]))
        step("lost", "continue", "-crash", "after-send", expected=77)
        completed = step("lost", "reconcile")
        check(completed["state"] == "TASK_STATE_COMPLETED" and completed["effects"] == 1, "lost effect receipt recovery failed")
        step("no-contract", "submit", "-dedup=false", "-lose-reply", expected=2)
        blocked = step("no-contract", "reconcile", "-dedup=false", expected=2)
        check(blocked["state"] == "UNKNOWN", "retry without a contract was allowed")
    print("PASS: A2A CLI normal/repeated run, lost creation reply, reviewed approval, effect crash, no-contract quarantine")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--mcp", required=True)
    parser.add_argument("--a2a", required=True)
    args = parser.parse_args()
    stdio(args.mcp)
    a2a(args.a2a)


if __name__ == "__main__":
    main()
