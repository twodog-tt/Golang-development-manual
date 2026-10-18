"""Package only the built site, with a commit-bound file manifest."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import tarfile


def package(site, output, release):
    if not re.fullmatch(r"[a-f0-9]{40}-[0-9]+-[0-9]+", release):
        raise ValueError("Invalid release ID")
    manifest_path = site / "release-manifest.json"
    files = {}
    for path in sorted(site.rglob("*")):
        if path.is_symlink():
            raise ValueError("Site must not contain symlinks")
        if path.is_file() and path != manifest_path:
            files[path.relative_to(site).as_posix()] = hashlib.sha256(path.read_bytes()).hexdigest()
    manifest_path.write_text(json.dumps({
        "release": release, "commit": release[:40],
        "domain": "knowledgebase.wenyiapp.cn", "files": files,
    }, ensure_ascii=False, indent=2) + "\n")
    with tarfile.open(output, "w:gz") as archive:
        for path in sorted(site.rglob("*")):
            if path.is_file():
                archive.add(path, arcname=path.relative_to(site).as_posix(), recursive=False)
    print(hashlib.sha256(output.read_bytes()).hexdigest())


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--site", type=Path, default=Path("site"))
    parser.add_argument("--output", type=Path, default=Path("release.tar.gz"))
    parser.add_argument("--release", required=True)
    args = parser.parse_args()
    package(args.site, args.output, args.release)
