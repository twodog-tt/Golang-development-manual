import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("receive", ROOT / "receive.py")
receive = importlib.util.module_from_spec(spec)
spec.loader.exec_module(receive)
RELEASE = "a" * 40 + "-123-1"


def bundle(extra=None, corrupt=False):
    files = {"index.html": b"home", "404.html": b"404", "search/search_index.json": b"{}", "sitemap.xml": b"xml"}
    manifest = {"release": RELEASE, "commit": RELEASE[:40], "domain": "knowledgebase.wenyiapp.cn",
                "files": {p: hashlib.sha256(v).hexdigest() for p, v in files.items()}}
    if corrupt:
        files["index.html"] = b"corrupted"
    files["release-manifest.json"] = json.dumps(manifest).encode()
    data = io.BytesIO()
    with tarfile.open(fileobj=data, mode="w:gz") as archive:
        for name, content in files.items():
            info = tarfile.TarInfo(name); info.size = len(content)
            archive.addfile(info, io.BytesIO(content))
        if extra:
            info, content = extra
            archive.addfile(info, io.BytesIO(content) if content is not None else None)
    return data.getvalue()


class DeploymentTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.base = Path(self.tmp.name).resolve()
        (self.base / "releases/previous").mkdir(parents=True)
        (self.base / ".deploy").mkdir()
        (self.base / "current").symlink_to("releases/previous")

    def activate(self, data, checksum=None):
        receive.deploy(self.base, RELEASE, checksum or hashlib.sha256(data).hexdigest(), io.BytesIO(data))

    def assert_previous(self):
        self.assertEqual((self.base / "current").resolve(), self.base / "releases/previous")

    def test_atomic_activation_and_rollback(self):
        self.activate(bundle())
        self.assertEqual((self.base / "current/index.html").read_bytes(), b"home")
        self.assertEqual((self.base / "current/index.html").stat().st_mode & 0o777, 0o644)
        receive.rollback(self.base, RELEASE)
        self.assert_previous()

    def test_bad_archive_checksum_keeps_current(self):
        with self.assertRaisesRegex(ValueError, "Archive checksum"):
            self.activate(bundle(), "0" * 64)
        self.assert_previous()

    def test_bad_file_checksum_keeps_current(self):
        with self.assertRaisesRegex(ValueError, "File checksum"):
            self.activate(bundle(corrupt=True))
        self.assert_previous()

    def test_archive_limits_keep_current(self):
        for limit in ("MAX_ARCHIVE", "MAX_CONTENT"):
            with self.subTest(limit=limit), patch.object(receive, limit, 1):
                with self.assertRaisesRegex(ValueError, "limit"):
                    self.activate(bundle())
                self.assert_previous()

    def test_manifest_identity_mismatch_keeps_current(self):
        data = bundle()
        with self.assertRaisesRegex(ValueError, "identity"):
            receive.deploy(self.base, "b" * 40 + "-123-1", hashlib.sha256(data).hexdigest(), io.BytesIO(data))
        self.assert_previous()

    def test_archive_traversal_and_symlink_rejected(self):
        for name, kind in [("../outside", tarfile.REGTYPE), ("/outside", tarfile.REGTYPE),
                           ("index.html", tarfile.REGTYPE), ("link", tarfile.SYMTYPE),
                           ("hardlink", tarfile.LNKTYPE)]:
            with self.subTest(name=name, kind=kind):
                info = tarfile.TarInfo(name); info.type = kind; info.linkname = "/etc/passwd"
                with self.assertRaises(ValueError):
                    self.activate(bundle((info, None if kind != tarfile.REGTYPE else b"")))
                self.assert_previous()
        self.assertFalse((self.base.parent / "outside").exists())

    def test_undeclared_file_rejected(self):
        with self.assertRaisesRegex(ValueError, "file list"):
            self.activate(bundle((tarfile.TarInfo("extra.html"), b"")))
        self.assert_previous()

    def test_rollback_cannot_override_newer_deployment(self):
        self.activate(bundle())
        receive.switch(self.base, "releases/previous")
        with self.assertRaisesRegex(ValueError, "different active"):
            receive.rollback(self.base, RELEASE)
        self.assert_previous()

    def test_shell_and_scp_are_rejected(self):
        for command in ["", "bash", "scp -t /tmp", "deploy ../../etc abc", "rollback " + RELEASE + "; id"]:
            with self.subTest(command=command), self.assertRaises(ValueError):
                receive.parse_command(command)
        self.assertEqual(receive.parse_command("deploy " + RELEASE + " " + "0" * 64)[0], "deploy")


if __name__ == "__main__":
    unittest.main()
