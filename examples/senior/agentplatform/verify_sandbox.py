"""Opt-in real Linux container checks. Missing Docker/image is a failure.
No production credentials, provider calls, image pulls or host-secret mounts.
"""
import json
import os
from pathlib import Path
import subprocess
import tempfile


def main():
    image = os.environ.get("AGENTPLATFORM_SANDBOX_IMAGE", "alpine:3.22.1")
    # Resolve a local tag once and use its immutable image ID for this run.
    image_id = subprocess.check_output(["docker", "image", "inspect", "--format", "{{.Id}}", image], text=True).strip()
    script = r'''set -eu
test "$(id -u)" = 65534
test -z "${AGENTPLATFORM_TEST_SECRET-}"
test ! -e /run/secrets/platform-key
test ! -e /var/run/docker.sock
test ! -e /run/signer.sock
if (echo x > /etc/plugin-write-probe) 2>/dev/null; then exit 11; fi
if wget -q -T 1 -O /tmp/network-probe http://192.0.2.1 2>/dev/null; then exit 12; fi
# Kernels may create DOWN tunnel devices even in an empty network namespace.
# Verify no routes, and exactly one UP interface (loopback).
test -z "$(ip route show)"
test -z "$(ip -6 route show)"
test "$(ip link show up | awk '/^[0-9]+:/ {print $2}')" = 'lo:'
awk '$2=="/" && $4 ~ /(^|,)ro(,|$)/ {found=1} END {exit !found}' /proc/mounts
test "$(awk '/^CapEff:/ {print $2}' /proc/self/status)" = 0000000000000000
test "$(awk '/^NoNewPrivs:/ {print $2}' /proc/self/status)" = 1
echo scratch > /tmp/local-probe
test "$(cat /tmp/local-probe)" = scratch
echo PASS
'''
    with tempfile.TemporaryDirectory(prefix="agentplatform-sandbox-") as temporary:
        root = Path(temporary)
        root.chmod(0o755)
        probe = root / "probe.sh"
        probe.write_text(script)
        probe.chmod(0o444)
        env = dict(os.environ, AGENTPLATFORM_TEST_SECRET="public-fault-probe-never-a-real-secret")
        result = subprocess.run([
            "docker", "run", "--rm", "--pull=never", "--network=none", "--read-only",
            "--user=65534:65534", "--cap-drop=ALL", "--security-opt=no-new-privileges",
            "--pids-limit=32", "--memory=64m", "--cpus=0.5",
            "--tmpfs=/tmp:rw,noexec,nosuid,nodev,size=1m,mode=1777",
            "--mount", f"type=bind,src={probe},dst=/probe.sh,readonly",
            image_id, "/bin/sh", "/probe.sh",
        ], env=env, capture_output=True, text=True, timeout=30)
        if result.returncode:
            raise RuntimeError(f"sandbox probe failed: {result.stdout}\n{result.stderr}")
        if result.stdout.strip() != "PASS":
            raise RuntimeError((result.stdout, result.stderr))
    print(json.dumps({"passed": True, "image_id": image_id, "checks": [
        "unprivileged_uid", "secret_not_inherited", "no_host_secret_or_sockets",
        "readonly_root", "no_external_route_or_up_interface", "no_capabilities_or_privilege_gain", "bounded_scratch_write"
    ]}))


if __name__ == "__main__":
    main()
