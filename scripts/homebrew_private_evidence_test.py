import datetime
import importlib.util
import os
import pathlib
import tempfile
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = ROOT / "scripts" / "homebrew_private_evidence.py"


def load_module(testcase):
    testcase.assertTrue(MODULE_PATH.exists(), "scripts/homebrew_private_evidence.py must exist")
    spec = importlib.util.spec_from_file_location("homebrew_private_evidence", MODULE_PATH)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class HomebrewPrivateEvidenceTest(unittest.TestCase):
    def setUp(self):
        self.module = load_module(self)
        self.commit = "0123456789abcdef0123456789abcdef01234567"
        self.valid = {
            "version": 1,
            "target_version": "v0.1.0",
            "commit": self.commit,
            "checked_at": "2026-07-01T00:00:00Z",
            "tap": {
                "repo": "hamardikan/homebrew-tap",
                "tap": "hamardikan/tap",
                "cask": "lazyss",
                "tapped": True,
            },
            "install": {
                "passed": True,
                "clean_homebrew_environment": True,
                "private_asset_downloaded": True,
                "token_env_name": "HOMEBREW_GITHUB_API_TOKEN",
                "installed_version": "v0.1.0",
            },
            "runtime": {
                "version_passed": True,
                "doctor_completed": True,
            },
            "safety": {
                "no_token_in_logs": True,
                "no_token_in_cask": True,
                "no_private_asset_url_recorded": True,
                "token_value_not_recorded": True,
            },
        }

    def test_validates_private_homebrew_install_evidence(self):
        events = self.module.validate_payload(self.valid, "v0.1.0", self.commit)

        self.assertEqual([event.level for event in events], ["ok", "ok", "ok"])
        self.assertIn("private Homebrew install evidence validated", events[0].message)

    def test_rejects_token_material_before_parsing(self):
        raw = '{"token":"ghp_abcdefghijklmnopqrstuvwxyz1234567890"}'

        events = self.module.validate_raw(raw, "v0.1.0", self.commit)

        self.assertEqual(len(events), 1)
        self.assertEqual(events[0].level, "fail")
        self.assertIn("credential material", events[0].message)

    def test_rejects_homebrew_token_assignment(self):
        raw = '{"note":"HOMEBREW_GITHUB_API_TOKEN=secret-value"}'

        events = self.module.validate_raw(raw, "v0.1.0", self.commit)

        self.assertEqual(events[0].level, "fail")
        self.assertIn("credential material", events[0].message)

    def test_template_writes_0600_and_false_defaults(self):
        now = datetime.datetime(2026, 7, 1, tzinfo=datetime.timezone.utc)
        with tempfile.TemporaryDirectory() as tmp:
            path = pathlib.Path(tmp) / "homebrew-private-evidence.json"

            self.module.write_template(path, "v0.1.0", self.commit, now)

            mode = os.stat(path).st_mode & 0o777
            self.assertEqual(mode, 0o600)
            payload = self.module.read_json(path)
            self.assertEqual(payload["target_version"], "v0.1.0")
            self.assertEqual(payload["commit"], self.commit)
            self.assertFalse(payload["install"]["passed"])
            self.assertEqual(payload["install"]["token_env_name"], "HOMEBREW_GITHUB_API_TOKEN")
            self.assertFalse(payload["safety"]["token_value_not_recorded"])
            with self.assertRaises(FileExistsError):
                self.module.write_template(path, "v0.1.0", self.commit, now)


if __name__ == "__main__":
    unittest.main()
