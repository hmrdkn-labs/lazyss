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


if __name__ == "__main__":
    unittest.main()
