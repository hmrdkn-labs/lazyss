import importlib.util
import pathlib
import tempfile
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = ROOT / "scripts" / "coverage_baseline.py"


def load_module(testcase):
    testcase.assertTrue(MODULE_PATH.exists(), "scripts/coverage_baseline.py must exist")
    spec = importlib.util.spec_from_file_location("coverage_baseline", MODULE_PATH)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class CoverageBaselineTest(unittest.TestCase):
    def setUp(self):
        self.module = load_module(self)

    def test_accepts_total_coverage_at_baseline(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            summary = root / "coverage.txt"
            baseline = root / "coverage.baseline"
            summary.write_text("github.com/hamardikan/lazyss/internal/domain\t100.0%\ntotal:\t(statements)\t57.7%\n", encoding="utf-8")
            baseline.write_text("57.7\n", encoding="utf-8")

            events = self.module.verify_coverage(summary, baseline)

            self.assertEqual([event.level for event in events], ["ok"])
            self.assertIn("57.7%", events[0].message)

    def test_rejects_total_coverage_below_baseline(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            summary = root / "coverage.txt"
            baseline = root / "coverage.baseline"
            summary.write_text("total:\t(statements)\t57.6%\n", encoding="utf-8")
            baseline.write_text("57.7\n", encoding="utf-8")

            events = self.module.verify_coverage(summary, baseline)

            self.assertEqual(events[0].level, "fail")
            self.assertIn("below baseline", events[0].message)
            self.assertIn("57.6%", events[0].message)
            self.assertIn("57.7%", events[0].message)

    def test_rejects_summary_without_total_line(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            summary = root / "coverage.txt"
            baseline = root / "coverage.baseline"
            summary.write_text("github.com/hamardikan/lazyss/internal/domain\t100.0%\n", encoding="utf-8")
            baseline.write_text("57.7\n", encoding="utf-8")

            events = self.module.verify_coverage(summary, baseline)

            self.assertEqual(events[0].level, "fail")
            self.assertIn("total coverage line", events[0].message)


if __name__ == "__main__":
    unittest.main()
