import importlib.util
import pathlib
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = ROOT / "scripts" / "workflow_policy.py"


def load_module(testcase):
    testcase.assertTrue(MODULE_PATH.exists(), "scripts/workflow_policy.py must exist")
    spec = importlib.util.spec_from_file_location("workflow_policy", MODULE_PATH)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class WorkflowPolicyTest(unittest.TestCase):
    def setUp(self):
        self.module = load_module(self)
        self.workflows = self.module.load_workflows(ROOT / ".github" / "workflows")

    def test_all_workflow_jobs_have_timeouts(self):
        missing = self.module.jobs_missing_timeouts(self.workflows)

        self.assertEqual([], missing)

    def test_non_release_workflows_are_read_only_and_do_not_use_secrets(self):
        violations = self.module.non_release_secret_or_write_violations(self.workflows)

        self.assertEqual([], violations)

    def test_release_workflow_is_tag_only_and_gates_goreleaser_on_readiness(self):
        release = self.workflows["release.yml"]

        self.assertTrue(self.module.release_is_semver_tag_only(release))
        self.assertTrue(self.module.step_runs_before(release, "Release readiness audit", "goreleaser/goreleaser-action@v7"))

    def test_release_candidate_summarizes_classifier_decision(self):
        release_candidate = self.workflows["release-candidate.yml"]

        self.assertTrue(self.module.step_writes_summary(release_candidate, "Decide whether release-candidate gates are required"))

    def test_pr_workflows_do_not_use_govulncheck_action_wrapper(self):
        violations = self.module.workflow_action_uses(self.workflows, "golang/govulncheck-action")

        self.assertEqual([], violations)

    def test_makefile_exposes_local_release_candidate_target(self):
        makefile = (ROOT / "Makefile").read_text(encoding="utf-8")

        self.assertIn(".PHONY: release-candidate-local", makefile)
        self.assertIn("release-candidate-local:", makefile)
        self.assertIn("release-snapshot", makefile)
        self.assertIn("homebrew-readiness", makefile)

    def test_build_matrix_writes_to_temporary_outputs(self):
        makefile = (ROOT / "Makefile").read_text(encoding="utf-8")

        self.assertIn('tmpdir="$$(mktemp -d)"', makefile)
        self.assertIn('-o "$$output"', makefile)


if __name__ == "__main__":
    unittest.main()
