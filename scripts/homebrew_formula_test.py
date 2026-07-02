import hashlib
import importlib.util
import pathlib
import tempfile
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = ROOT / "scripts" / "homebrew_formula.py"


def load_module():
    spec = importlib.util.spec_from_file_location("homebrew_formula", MODULE_PATH)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class HomebrewFormulaTest(unittest.TestCase):
    def setUp(self):
        self.module = load_module()

    def write_dist(self, path, files):
        path.mkdir()
        lines = []
        for name, content in files.items():
            artifact = path / name
            artifact.write_bytes(content)
            if name.endswith(".tar.gz"):
                lines.append(f"{hashlib.sha256(content).hexdigest()}  {name}")
        (path / "checksums.txt").write_text("\n".join(lines) + "\n", encoding="utf-8")

    def complete_files(self):
        return {
            "lazyss_0.1.0_darwin_amd64.tar.gz": b"darwin-amd64",
            "lazyss_0.1.0_darwin_arm64.tar.gz": b"darwin-arm64",
            "lazyss_0.1.0_linux_amd64.tar.gz": b"linux-amd64",
            "lazyss_0.1.0_linux_arm64.tar.gz": b"linux-arm64",
            "lazyss_0.1.0_windows_amd64.zip": b"windows-amd64",
        }

    def test_generates_public_formula_from_dist(self):
        with tempfile.TemporaryDirectory() as tmp:
            dist = pathlib.Path(tmp) / "dist"
            files = self.complete_files()
            self.write_dist(dist, files)

            formula = self.module.render_formula(dist, "v0.1.0")

            self.assertIn("class Lazyss < Formula", formula)
            self.assertIn('version "0.1.0"', formula)
            self.assertIn('license "MIT"', formula)
            self.assertIn('homepage "https://github.com/hmrdkn-labs/lazyss"', formula)
            self.assertIn('bin.install "lazyss"', formula)
            self.assertIn('system "#{bin}/lazyss", "--version"', formula)
            for name, content in files.items():
                if not name.endswith(".tar.gz"):
                    continue
                self.assertIn(f"https://github.com/hmrdkn-labs/lazyss/releases/download/v0.1.0/{name}", formula)
                self.assertIn(hashlib.sha256(content).hexdigest(), formula)
            self.assertNotIn("Authorization: Bearer", formula)
            self.assertNotIn("HOMEBREW_GITHUB_API_TOKEN", formula)

    def test_rejects_missing_formula_archive(self):
        with tempfile.TemporaryDirectory() as tmp:
            dist = pathlib.Path(tmp) / "dist"
            files = self.complete_files()
            files.pop("lazyss_0.1.0_linux_arm64.tar.gz")
            self.write_dist(dist, files)

            with self.assertRaisesRegex(ValueError, "linux/arm64"):
                self.module.render_formula(dist, "v0.1.0")

    def test_rejects_missing_checksum(self):
        with tempfile.TemporaryDirectory() as tmp:
            dist = pathlib.Path(tmp) / "dist"
            files = self.complete_files()
            self.write_dist(dist, files)
            checksums = dist / "checksums.txt"
            checksums.write_text(
                "\n".join(line for line in checksums.read_text(encoding="utf-8").splitlines() if "darwin_amd64" not in line)
                + "\n",
                encoding="utf-8",
            )

            with self.assertRaisesRegex(ValueError, "missing checksum"):
                self.module.render_formula(dist, "v0.1.0")


if __name__ == "__main__":
    unittest.main()
