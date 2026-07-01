import pathlib
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[1]
RELEASE_READINESS = ROOT / "scripts" / "release-readiness.sh"


class ReleaseReadinessTest(unittest.TestCase):
    def test_requires_private_homebrew_install_evidence(self):
        text = RELEASE_READINESS.read_text(encoding="utf-8")

        self.assertIn("LAZYSS_HOMEBREW_PRIVATE_EVIDENCE", text)
        self.assertIn("homebrew_private_evidence.py", text)
        self.assertIn("private Homebrew install evidence file is not provided", text)

    def test_makefile_exposes_homebrew_private_evidence_template(self):
        text = (ROOT / "Makefile").read_text(encoding="utf-8")

        self.assertIn(".PHONY: homebrew-private-evidence-template", text)
        self.assertIn("scripts/homebrew_private_evidence.py template", text)


if __name__ == "__main__":
    unittest.main()
