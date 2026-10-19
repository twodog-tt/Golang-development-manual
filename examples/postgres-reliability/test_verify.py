"""Report lifecycle checks; standard library only, no Docker connection."""
import contextlib
import io
import json
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import verify


class ReportTests(unittest.TestCase):
    def run_fixture(self, path, failure=None):
        class FakeLab:
            token = "current-run"
            resources = []
            evidence = [{"scenario": "fixture", "passed": True}]

            def setup(self, image):
                if failure == "startup":
                    raise RuntimeError("startup failed")

            def ledger(self):
                if failure == "scenario":
                    raise RuntimeError("scenario failed")

            def cleanup(self):
                if failure == "cleanup":
                    raise RuntimeError("cleanup failed")

            def sql(self, *args):
                return "fixture PostgreSQL"

        with patch.object(verify, "Lab", FakeLab), patch.object(verify, "command", return_value=SimpleNamespace(stdout='[{"Id":"fixture-image"}]')):
            with contextlib.redirect_stdout(io.StringIO()):
                verify.main(["--only", "ledger", "--report", str(path)])

    def test_failed_run_cannot_retain_previous_success(self):
        for phase in ("startup", "scenario", "cleanup"):
            with self.subTest(phase=phase), tempfile.TemporaryDirectory() as tmp:
                path = Path(tmp) / "result.json"
                path.write_text('{"passed":true,"run_id":"old-run"}')
                with self.assertRaisesRegex(RuntimeError, phase + " failed"):
                    self.run_fixture(path, phase)
                self.assertFalse(path.exists())
                self.assertEqual(list(Path(tmp).iterdir()), [])

    def test_success_replaces_stale_report_and_records_selection(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "nested" / "result.json"
            self.run_fixture(path)
            result = json.loads(path.read_text())
            self.assertTrue(result["passed"])
            self.assertEqual((result["run_id"], result["selection"]), ("current-run", "ledger"))
            self.assertEqual(set(result["source_sha256"]), set(verify.SOURCE_FILES))
            self.assertEqual(list(path.parent.iterdir()), [path])
            path.write_text('{"passed":true,"run_id":"old-run"}')
            self.run_fixture(path)
            self.assertEqual(json.loads(path.read_text())["run_id"], "current-run")

    def test_publish_failure_leaves_no_partial_success(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "result.json"
            with patch.object(verify.os, "replace", side_effect=OSError("publish failed")):
                with self.assertRaisesRegex(OSError, "publish failed"):
                    self.run_fixture(path)
            self.assertFalse(path.exists())
            self.assertEqual(list(Path(tmp).iterdir()), [])

    def test_cleanup_reports_failures_after_attempting_every_owned_container(self):
        lab = verify.Lab()
        lab.resources = ["owned-pg", "owned-redis"]
        results = [SimpleNamespace(returncode=1, stderr="daemon unavailable"), SimpleNamespace(returncode=0, stderr="")]
        with patch.object(verify, "command", side_effect=results) as command:
            with self.assertRaisesRegex(RuntimeError, "cleanup failed.*owned-redis"):
                lab.cleanup()
        self.assertEqual(command.call_count, 2)

    def test_cleanup_accepts_already_removed_owned_container(self):
        lab = verify.Lab()
        lab.resources = ["owned-absent"]
        with patch.object(verify, "command", return_value=SimpleNamespace(returncode=1, stderr="No such container: owned-absent")):
            lab.cleanup()


if __name__ == "__main__":
    unittest.main()
