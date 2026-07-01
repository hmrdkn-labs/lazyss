import json
import pathlib
import subprocess
import tempfile
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "branch_protection_plan.py"


class BranchProtectionPlanTest(unittest.TestCase):
    def test_writes_reviewable_branch_protection_json_and_markdown(self):
        with tempfile.TemporaryDirectory() as tmp:
            out = pathlib.Path(tmp)
            result = subprocess.run(
                [
                    "python3",
                    str(SCRIPT),
                    "--repo",
                    "hamardikan/lazyss",
                    "--branch",
                    "main",
                    "--required-check",
                    "ci-required",
                    "--json-output",
                    str(out / "branch-protection.json"),
                    "--markdown-output",
                    str(out / "branch-protection.md"),
                ],
                cwd=ROOT,
                text=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                check=False,
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("wrote", result.stdout)

            payload = json.loads((out / "branch-protection.json").read_text(encoding="utf-8"))
            self.assertEqual(payload["required_status_checks"]["strict"], True)
            self.assertEqual(payload["required_status_checks"]["contexts"], ["ci-required"])
            self.assertEqual(payload["enforce_admins"], True)
            self.assertEqual(payload["required_linear_history"], True)
            self.assertEqual(payload["allow_force_pushes"], False)
            self.assertEqual(payload["allow_deletions"], False)
            self.assertEqual(payload["required_pull_request_reviews"]["required_approving_review_count"], 1)
            self.assertEqual(payload["required_conversation_resolution"], True)

            markdown = (out / "branch-protection.md").read_text(encoding="utf-8")
            self.assertIn("hamardikan/lazyss", markdown)
            self.assertIn("main", markdown)
            self.assertIn("ci-required", markdown)
            self.assertIn("gh api --method PUT repos/hamardikan/lazyss/branches/main/protection", markdown)
            self.assertIn("./scripts/branch-protection-readiness.sh", markdown)
            self.assertIn("requires explicit owner approval", markdown)

    def test_rejects_secret_like_values_in_inputs(self):
        with tempfile.TemporaryDirectory() as tmp:
            out = pathlib.Path(tmp)
            result = subprocess.run(
                [
                    "python3",
                    str(SCRIPT),
                    "--repo",
                    "hamardikan/lazyss",
                    "--required-check",
                    "github_pat_secret",
                    "--json-output",
                    str(out / "branch-protection.json"),
                ],
                cwd=ROOT,
                text=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                check=False,
            )

            self.assertNotEqual(result.returncode, 0)
            self.assertIn("credential-like", result.stderr)
            self.assertFalse((out / "branch-protection.json").exists())


if __name__ == "__main__":
    unittest.main()
