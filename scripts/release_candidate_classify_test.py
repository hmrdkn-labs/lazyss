import importlib.util
import pathlib
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = ROOT / "scripts" / "release_candidate_classify.py"


def load_module(testcase):
    testcase.assertTrue(MODULE_PATH.exists(), "scripts/release_candidate_classify.py must exist")
    spec = importlib.util.spec_from_file_location("release_candidate_classify", MODULE_PATH)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class ReleaseCandidateClassifyTest(unittest.TestCase):
    def setUp(self):
        self.module = load_module(self)

    def test_non_pull_request_events_always_run(self):
        decision = self.module.classify("push", [], [])

        self.assertTrue(decision.should_run)
        self.assertEqual(decision.reason, "push event")

    def test_release_labels_force_release_candidate_gates(self):
        decision = self.module.classify("pull_request", ["docs", "release-candidate"], ["README.md"])

        self.assertTrue(decision.should_run)
        self.assertEqual(decision.reason, "release label")

    def test_policy_and_quality_files_are_release_relevant(self):
        files = [
            ".github/workflows/ci.yml",
            ".golangci.yml",
            "coverage.baseline",
            ".github/dependabot.yml",
            ".github/CODEOWNERS",
            ".github/pull_request_template.md",
            "docs/runbooks/quality-gates.md",
        ]

        for path in files:
            with self.subTest(path=path):
                decision = self.module.classify("pull_request", [], [path])
                self.assertTrue(decision.should_run)
                self.assertEqual(decision.reason, "release-relevant files")

    def test_ordinary_docs_do_not_run_without_label(self):
        decision = self.module.classify("pull_request", [], ["docs/notes/idea.md", "README.md"])

        self.assertFalse(decision.should_run)
        self.assertIn("not release-relevant", decision.reason)


if __name__ == "__main__":
    unittest.main()
