#!/usr/bin/env python3
"""Offline Go experiment: module-cache verification does not verify build inputs.

Only self-generated fixture modules are used. GOSUMDB=off is scoped to subprocesses
using an isolated file:// proxy/cache; no shell/global Go configuration is changed.
"""

import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import zipfile


REPO = Path(__file__).resolve().parents[3]
GO = os.environ.get("GO_BIN") or ("/opt/homebrew/bin/go" if Path("/opt/homebrew/bin/go").exists() else shutil.which("go"))
if not GO:
    raise SystemExit("Go 1.24.2+ is required")


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def write(path, text):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text)


with tempfile.TemporaryDirectory(prefix="release-input-drill-") as directory:
    # macOS /var is a symlink; Go workspace membership compares physical paths.
    tmp = Path(directory).resolve()
    env = os.environ.copy()
    env.pop("GOROOT", None)
    env.update(GOPROXY=(tmp / "proxy").as_uri(), GOSUMDB="off", GONOPROXY="none",
               GONOSUMDB="*", GOPRIVATE="", GOVCS="*:off", GOTOOLCHAIN="local",
               GOWORK="off", GOFLAGS="", GOPATH=str(tmp / "gopath"),
               GOMODCACHE=str(tmp / "modcache"))

    def command(args, cwd, expected=0, extra=None):
        result = subprocess.run(args, cwd=cwd, env=env | (extra or {}), text=True,
                                capture_output=True, timeout=60)
        require(result.returncode == expected,
                f"{args}: expected exit {expected}, got {result.returncode}\n{result.stdout}\n{result.stderr}")
        return result.stdout.strip()

    # The fingerprint tool uses only the standard library; build with all network
    # fetching disabled even though the surrounding repository has dependencies.
    binary = tmp / "fingerprint"
    command([GO, "build", "-o", str(binary), "./examples/senior/releaseadmission/cmd/fingerprint"],
            REPO, extra={"GOPROXY": "off"})
    module = "example.test/dependency"
    module_file = f"module {module}\n\ngo 1.24.2\n"
    source = 'package dependency\nfunc Message() string { return "original" }\n'
    proxy = tmp / "proxy" / module / "@v"
    write(proxy / "v1.0.0.mod", module_file)
    write(proxy / "v1.0.0.info", '{"Version":"v1.0.0","Time":"2026-01-01T00:00:00Z"}')
    write(proxy / "list", "v1.0.0\n")
    with zipfile.ZipFile(proxy / "v1.0.0.zip", "w") as archive:
        archive.writestr(f"{module}@v1.0.0/go.mod", module_file)
        archive.writestr(f"{module}@v1.0.0/message.go", source)

    def app(root, replacement=""):
        write(root / "go.mod", f"module example.test/app\n\ngo 1.24.2\n\nrequire {module} v1.0.0\n" + replacement)
        write(root / "main.go", 'package main\nimport ("fmt"; "example.test/dependency")\nfunc main(){ fmt.Println(dependency.Message()) }\n')

    def fingerprint(root):
        return json.loads(command([str(binary), "-root", str(root)], REPO))["digest"]

    def verify(cwd, extra=None):
        output = command([GO, "mod", "verify"], cwd, extra=extra)
        require(output == "all modules verified", f"Unexpected verify output: {output}")

    def mutate_and_check(name, root, cwd, changed, run_args, extra=None):
        before = command([GO, "run", *run_args, "."], cwd, extra=extra)
        verify(cwd, extra)
        digest = fingerprint(root)
        write(changed, changed.read_text().replace('"original"', '"tampered"'))
        verify(cwd, extra)
        after = command([GO, "run", *run_args, "."], cwd, extra=extra)
        require((before, after) == ("original", "tampered"), f"{name}: actual program did not change")
        result = json.loads(command([str(binary), "-root", str(root), "-expected", digest], REPO, expected=1))
        require(result["matches"] is False, f"{name}: changed build input accepted")
        return {"case": name, "go_mod_verify_passed_after_tamper": True,
                "program_before": before, "program_after": after, "input_change_blocked": True}, digest

    vendor = tmp / "vendor-case"
    app(vendor)
    command([GO, "mod", "download", module], vendor)
    command([GO, "mod", "vendor"], vendor)
    vendor_result, baseline = mutate_and_check("vendor", vendor, vendor,
                                              vendor / "vendor" / module / "message.go", ["-mod=vendor"])
    command([GO, "mod", "vendor"], vendor)
    require(fingerprint(vendor) == baseline, "re-vendoring from verified module cache did not restore inputs")
    vendor_result["revendor_restored_inputs"] = True

    local = tmp / "replace-case"
    app(local, f"replace {module} => ./local\n")
    write(local / "local" / "go.mod", module_file)
    write(local / "local" / "message.go", source)
    local_result, _ = mutate_and_check("local-replace", local, local,
                                       local / "local" / "message.go", ["-mod=mod"])

    workspace = tmp / "workspace-case"
    app(workspace / "app")
    write(workspace / "workdep" / "go.mod", module_file)
    write(workspace / "workdep" / "message.go", source)
    workfile = workspace / "go.work"
    write(workfile, "go 1.24.2\n\nuse (\n ./app\n ./workdep\n)\n")
    workspace_result, _ = mutate_and_check("workspace", workspace, workspace / "app",
                                          workspace / "workdep" / "message.go", [],
                                          {"GOWORK": str(workfile)})
    print(json.dumps({"passed": True, "network_dependencies": 0,
                      "cases": [vendor_result, local_result, workspace_result]}, indent=2))
