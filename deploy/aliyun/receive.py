"""Root-owned SSH forced command, executed as the unprivileged kbdeploy user.

Accepts a gzipped site on stdin via `deploy RELEASE SHA256`, or `rollback RELEASE`.
No shell, SCP, port forwarding, Nginx configuration or certificate access is needed.
"""
import fcntl
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import shlex
import shutil
import sys
import tarfile
import tempfile

BASE = Path("/var/www/knowledgebase")
MAX_ARCHIVE = 50 * 1024 * 1024
MAX_CONTENT = 200 * 1024 * 1024


def parse_command(command):
    parts = shlex.split(command)
    if len(parts) not in (2, 3) or not re.fullmatch(r"[a-f0-9]{40}-[0-9]+-[0-9]+", parts[1]):
        raise ValueError("Invalid command or release ID")
    if parts[0] == "deploy" and len(parts) == 3 and re.fullmatch(r"[a-f0-9]{64}", parts[2]):
        return parts
    if parts[0] == "rollback" and len(parts) == 2:
        return parts
    raise ValueError("Only deploy and rollback are permitted")


def switch(base, target):
    # The temporary link is in the same filesystem; os.replace is atomic.
    link = base / ".current.next"
    link.unlink(missing_ok=True)
    link.symlink_to(target)
    os.replace(link, base / "current")


def extract_verified(archive, destination, release):
    with tarfile.open(archive, "r:gz") as bundle:
        members = bundle.getmembers()
        names = set()
        total = 0
        for item in members:
            path = PurePosixPath(item.name)
            if (not item.isfile() or path.is_absolute() or ".." in path.parts
                    or str(path) != item.name or item.name in names or item.size < 0):
                raise ValueError("Unsafe or duplicate archive entry")
            names.add(item.name)
            total += item.size
        if total > MAX_CONTENT or len(members) > 10000:
            raise ValueError("Uncompressed site exceeds limits")
        for item in members:
            target = destination / item.name
            target.parent.mkdir(parents=True, exist_ok=True)
            with bundle.extractfile(item) as source, target.open("xb") as output:
                shutil.copyfileobj(source, output)
            target.chmod(0o644)
    manifest = json.loads((destination / "release-manifest.json").read_text())
    if (manifest["release"] != release or manifest["commit"] != release[:40]
            or manifest["domain"] != "knowledgebase.wenyiapp.cn"):
        raise ValueError("Release identity mismatch")
    if names != set(manifest["files"]) | {"release-manifest.json"}:
        raise ValueError("Manifest file list mismatch")
    for name, expected in manifest["files"].items():
        if hashlib.sha256((destination / name).read_bytes()).hexdigest() != expected:
            raise ValueError("File checksum mismatch: " + name)
    for required in ("index.html", "404.html", "search/search_index.json", "sitemap.xml"):
        if required not in manifest["files"]:
            raise ValueError("Required site file missing: " + required)
    return len(manifest["files"])


def deploy(base, release, expected, stream):
    releases = base / "releases"
    destination = releases / release
    state = base / ".deploy" / (release + ".json")
    if destination.exists() or state.exists():
        raise ValueError("Release already exists; rerun the workflow for a new attempt")
    previous = (base / "current").resolve(strict=True)
    if previous.parent != releases.resolve():
        raise ValueError("Current version is outside releases")
    with tempfile.TemporaryDirectory(prefix=".incoming-", dir=releases) as tmp:
        staging = Path(tmp)
        archive = staging / "bundle.tar.gz"
        digest = hashlib.sha256()
        size = 0
        with archive.open("wb") as output:
            while chunk := stream.read(1024 * 1024):
                size += len(chunk)
                if size > MAX_ARCHIVE:
                    raise ValueError("Archive exceeds limit")
                digest.update(chunk)
                output.write(chunk)
        if digest.hexdigest() != expected:
            raise ValueError("Archive checksum mismatch")
        content = staging / "site"
        content.mkdir(mode=0o755)
        count = extract_verified(archive, content, release)
        os.rename(content, destination)
    state.write_text(json.dumps({"previous": str(previous), "release": release}) + "\n")
    switch(base, "releases/" + release)
    print(json.dumps({"release": release, "previous": str(previous), "verified_files": count}))


def rollback(base, release):
    if (base / "current").resolve() != (base / "releases" / release).resolve():
        raise ValueError("Refusing to roll back a different active version")
    state = json.loads((base / ".deploy" / (release + ".json")).read_text())
    previous = Path(state["previous"]).resolve(strict=True)
    if previous.parent != (base / "releases").resolve():
        raise ValueError("Invalid rollback target")
    switch(base, "releases/" + previous.name)
    print(json.dumps({"rolled_back": release, "active": previous.name}))


def main():
    os.umask(0o022)
    command = parse_command(os.environ.get("SSH_ORIGINAL_COMMAND", ""))
    state_dir = BASE / ".deploy"
    state_dir.mkdir(mode=0o700, exist_ok=True)
    with (state_dir / "lock").open("a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        if command[0] == "deploy":
            deploy(BASE, command[1], command[2], sys.stdin.buffer)
        else:
            rollback(BASE, command[1])


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print("Deployment rejected: " + str(error), file=sys.stderr)
        sys.exit(1)
