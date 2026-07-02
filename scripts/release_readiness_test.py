import pathlib
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[1]
RELEASE_READINESS = ROOT / "scripts" / "release-readiness.sh"


class ReleaseReadinessTest(unittest.TestCase):
    def test_requires_public_repo_for_release(self):
        text = RELEASE_READINESS.read_text(encoding="utf-8")

        self.assertIn("public release requires repository visibility change", text)
        self.assertIn("Homebrew readiness", text)
        self.assertNotIn("LAZYSS_HOMEBREW_PRIVATE_EVIDENCE", text)
        self.assertNotIn("homebrew_private_evidence.py", text)

    def test_makefile_does_not_expose_private_homebrew_evidence_template(self):
        text = (ROOT / "Makefile").read_text(encoding="utf-8")

        self.assertNotIn(".PHONY: homebrew-private-evidence-template", text)
        self.assertNotIn("scripts/homebrew_private_evidence.py template", text)

    def test_readme_release_commands_include_all_evidence_inputs(self):
        text = (ROOT / "README.md").read_text(encoding="utf-8")

        self.assertIn("LAZYSS_LIVE_SMOKE_EVIDENCE=live-smoke-evidence.json", text)
        self.assertIn("verify a clean Homebrew formula install", text)
        self.assertIn("make live-smoke-evidence-template", text)
        self.assertIn("make release-approval-plan", text)


if __name__ == "__main__":
    unittest.main()
