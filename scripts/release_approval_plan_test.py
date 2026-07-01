import pathlib
import subprocess
import tempfile
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "release_approval_plan.py"


class ReleaseApprovalPlanTest(unittest.TestCase):
    def test_writes_reviewable_release_approval_markdown(self):
        with tempfile.TemporaryDirectory() as tmp:
            out = pathlib.Path(tmp) / "release-approval.md"
            result = subprocess.run(
                [
                    "python3",
                    str(SCRIPT),
                    "--repo",
                    "hamardikan/lazyss",
                    "--tap-repo",
                    "hamardikan/homebrew-tap",
                    "--tap",
                    "hamardikan/tap",
                    "--target-version",
                    "v0.1.0",
                    "--markdown-output",
                    str(out),
                ],
                cwd=ROOT,
                text=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                check=False,
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("wrote", result.stdout)

            markdown = out.read_text(encoding="utf-8")
            self.assertIn("# LazySS Release Approval Plan", markdown)
            self.assertIn("Repository: `hamardikan/lazyss`", markdown)
            self.assertIn("Tap repository: `hamardikan/homebrew-tap`", markdown)
            self.assertIn("Tap: `hamardikan/tap`", markdown)
            self.assertIn("Target version: `v0.1.0`", markdown)
            self.assertIn("local and read-only", markdown)
            self.assertIn("does not create repositories, secrets, branch protection, tags, releases, or public assets", markdown)
            self.assertIn("Do not paste token values", markdown)
            self.assertIn("session-manager-plugin", markdown)
            self.assertIn("make branch-protection-plan", markdown)
            self.assertIn("HOMEBREW_TAP_GITHUB_TOKEN", markdown)
            self.assertIn("LAZYSS_RELEASE_READINESS_GITHUB_TOKEN", markdown)
            self.assertIn("LAZYSS_LIVE_SMOKE_EVIDENCE_JSON", markdown)
            self.assertIn("LAZYSS_HOMEBREW_PRIVATE_EVIDENCE_JSON", markdown)
            self.assertIn("make live-smoke-evidence-template", markdown)
            self.assertIn("make homebrew-private-evidence-template", markdown)
            self.assertIn("after post-publish private cask", markdown)
            self.assertIn("LAZYSS_REQUIRE_HOMEBREW_PRIVATE_EVIDENCE=1", markdown)
            self.assertIn("LAZYSS_LIVE_SMOKE_EVIDENCE=live-smoke-evidence.json", markdown)
            self.assertIn("./scripts/release-readiness.sh", markdown)
            self.assertIn("git tag v0.1.0", markdown)

    def test_rejects_credential_like_values_in_inputs(self):
        with tempfile.TemporaryDirectory() as tmp:
            out = pathlib.Path(tmp) / "release-approval.md"
            result = subprocess.run(
                [
                    "python3",
                    str(SCRIPT),
                    "--repo",
                    "github_pat_secret",
                    "--markdown-output",
                    str(out),
                ],
                cwd=ROOT,
                text=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                check=False,
            )

            self.assertNotEqual(result.returncode, 0)
            self.assertIn("credential-like", result.stderr)
            self.assertFalse(out.exists())

    def test_makefile_exposes_release_approval_plan_target(self):
        text = (ROOT / "Makefile").read_text(encoding="utf-8")

        self.assertIn(".PHONY: release-approval-plan", text)
        self.assertIn("scripts/release_approval_plan.py", text)
        self.assertIn("--markdown-output release-approval.md", text)

    def test_gitignore_ignores_generated_release_approval_markdown(self):
        text = (ROOT / ".gitignore").read_text(encoding="utf-8")

        self.assertIn("/release-approval.md", text)


if __name__ == "__main__":
    unittest.main()
