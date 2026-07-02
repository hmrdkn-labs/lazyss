import pathlib
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[1]
RELEASE_WORKFLOW = ROOT / ".github" / "workflows" / "release.yml"


class ReleaseWorkflowTest(unittest.TestCase):
    def test_release_workflow_uploads_readiness_reports_even_after_audit_failure(self):
        text = RELEASE_WORKFLOW.read_text(encoding="utf-8")

        self.assertIn("LAZYSS_RELEASE_READINESS_JSON: release-readiness.json", text)
        self.assertIn("LAZYSS_RELEASE_READINESS_MARKDOWN: release-readiness.md", text)
        self.assertIn("actions/upload-artifact@v7", text)
        self.assertIn("if: always()", text)
        self.assertIn("release-readiness-${{ github.ref_name }}", text)
        self.assertIn("release-readiness.json", text)
        self.assertIn("release-readiness.md", text)

    def test_release_workflow_requires_public_homebrew_tap_upload(self):
        text = RELEASE_WORKFLOW.read_text(encoding="utf-8")

        self.assertNotIn("LAZYSS_HOMEBREW_PRIVATE_EVIDENCE_JSON", text)
        self.assertNotIn("homebrew-private-evidence.json", text)
        self.assertNotIn("Write optional private Homebrew evidence", text)
        self.assertNotIn("LAZYSS_REQUIRE_HOMEBREW_PRIVATE_EVIDENCE", text)
        self.assertIn('LAZYSS_REQUIRE_HOMEBREW_TAP_UPLOAD: "1"', text)
        self.assertIn("HOMEBREW_TAP_GITHUB_TOKEN", text)
        self.assertIn("Checkout Homebrew tap", text)
        self.assertIn("repository: hmrdkn-labs/homebrew-tap", text)
        self.assertIn("scripts/homebrew_formula.py generate", text)
        self.assertIn("homebrew-tap/Formula/lazyss.rb", text)
        self.assertLess(text.index("goreleaser/goreleaser-action"), text.index("Update Homebrew formula"))


if __name__ == "__main__":
    unittest.main()
