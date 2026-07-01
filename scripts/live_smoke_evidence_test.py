import datetime
import importlib.util
import os
import pathlib
import tempfile
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = ROOT / "scripts" / "live_smoke_evidence.py"


def load_module():
    spec = importlib.util.spec_from_file_location("live_smoke_evidence", MODULE_PATH)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class LiveSmokeEvidenceTest(unittest.TestCase):
    def setUp(self):
        self.module = load_module()
        self.commit = "0123456789abcdef0123456789abcdef01234567"
        self.valid = {
            "version": 1,
            "target_version": "v0.1.0",
            "commit": self.commit,
            "checked_at": "2026-07-01T00:00:00Z",
            "ssh": {
                "passed": True,
                "host_label": "release-ssh-host",
                "config_mutated": False,
            },
            "aws_ssm": {
                "passed": True,
                "doctor_passed": True,
                "inventory_passed": True,
                "session_launch_passed": True,
                "degraded_ssh_preserved": True,
                "region": "ap-southeast-1",
                "target_label": "release-ssm-node",
            },
            "safety": {
                "no_secrets_observed": True,
                "state_mode_0600": True,
                "failed_connection_preserved_last_success": True,
            },
        }

    def test_validates_release_evidence(self):
        events = self.module.validate_payload(self.valid, "v0.1.0", self.commit)

        self.assertEqual([event.level for event in events], ["ok", "ok", "ok", "ok"])
        self.assertIn("live smoke evidence validated", events[0].message)

    def test_rejects_credential_material_before_parsing(self):
        raw = '{"token":"github_pat_abcdefghijklmnopqrstuvwxyz1234567890"}'

        events = self.module.validate_raw(raw, "v0.1.0", self.commit)

        self.assertEqual(len(events), 1)
        self.assertEqual(events[0].level, "fail")
        self.assertIn("credential material", events[0].message)

    def test_template_writes_0600_and_refuses_overwrite(self):
        now = datetime.datetime(2026, 7, 1, tzinfo=datetime.timezone.utc)
        with tempfile.TemporaryDirectory() as tmp:
            path = pathlib.Path(tmp) / "live-smoke-evidence.json"

            self.module.write_template(path, "v0.1.0", self.commit, now)

            mode = os.stat(path).st_mode & 0o777
            self.assertEqual(mode, 0o600)
            payload = self.module.read_json(path)
            self.assertEqual(payload["target_version"], "v0.1.0")
            self.assertEqual(payload["commit"], self.commit)
            self.assertFalse(payload["ssh"]["passed"])
            self.assertFalse(payload["aws_ssm"]["passed"])
            with self.assertRaises(FileExistsError):
                self.module.write_template(path, "v0.1.0", self.commit, now)


if __name__ == "__main__":
    unittest.main()
