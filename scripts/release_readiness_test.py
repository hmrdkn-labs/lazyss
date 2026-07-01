import pathlib
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[1]
RELEASE_READINESS = ROOT / "scripts" / "release-readiness.sh"


class ReleaseReadinessTest(unittest.TestCase):
    def test_supports_post_publish_private_homebrew_install_evidence(self):
        text = RELEASE_READINESS.read_text(encoding="utf-8")

        self.assertIn("LAZYSS_HOMEBREW_PRIVATE_EVIDENCE", text)
        self.assertIn("LAZYSS_REQUIRE_HOMEBREW_PRIVATE_EVIDENCE", text)
        self.assertIn("homebrew_private_evidence.py", text)
        self.assertIn("private Homebrew install evidence file is not provided", text)
        self.assertIn("not required for pre-publish readiness", text)

    def test_makefile_exposes_homebrew_private_evidence_template(self):
        text = (ROOT / "Makefile").read_text(encoding="utf-8")

        self.assertIn(".PHONY: homebrew-private-evidence-template", text)
        self.assertIn("scripts/homebrew_private_evidence.py template", text)

    def test_readme_release_commands_include_all_evidence_inputs(self):
        text = (ROOT / "README.md").read_text(encoding="utf-8")

        self.assertIn("LAZYSS_LIVE_SMOKE_EVIDENCE=live-smoke-evidence.json", text)
        self.assertIn("post-publish gate", text)
        self.assertIn("make live-smoke-evidence-template", text)
        self.assertIn("make homebrew-private-evidence-template", text)
        self.assertIn("make release-approval-plan", text)


if __name__ == "__main__":
    unittest.main()
