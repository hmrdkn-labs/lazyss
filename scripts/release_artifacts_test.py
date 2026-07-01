import hashlib
import importlib.util
import pathlib
import tempfile
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = ROOT / "scripts" / "release_artifacts.py"


def load_module():
    spec = importlib.util.spec_from_file_location("release_artifacts", MODULE_PATH)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class ReleaseArtifactsTest(unittest.TestCase):
    def setUp(self):
        self.module = load_module()

    def write_dist(self, path, files):
        path.mkdir()
        lines = []
        for name, content in files.items():
            artifact = path / name
            artifact.write_bytes(content)
            lines.append(f"{hashlib.sha256(content).hexdigest()}  {name}")
        (path / "checksums.txt").write_text("\n".join(lines) + "\n", encoding="utf-8")

    def complete_files(self):
        return {
            "lazyss_0.1.0_darwin_amd64.tar.gz": b"darwin-amd64",
            "lazyss_0.1.0_darwin_arm64.tar.gz": b"darwin-arm64",
            "lazyss_0.1.0_linux_amd64.tar.gz": b"linux-amd64",
            "lazyss_0.1.0_linux_arm64.tar.gz": b"linux-arm64",
            "lazyss_0.1.0_windows_amd64.zip": b"windows-amd64",
            "lazyss_0.1.0_windows_arm64.zip": b"windows-arm64",
        }

    def test_accepts_complete_archive_set_with_matching_checksums(self):
        with tempfile.TemporaryDirectory() as tmp:
            dist = pathlib.Path(tmp) / "dist"
            self.write_dist(dist, self.complete_files())

            events = self.module.verify_dist(dist)

            self.assertEqual([event.level for event in events], ["ok", "ok", "ok"])
            self.assertIn("all required archives are present", events[0].message)

    def test_rejects_missing_platform_archive(self):
        with tempfile.TemporaryDirectory() as tmp:
            dist = pathlib.Path(tmp) / "dist"
            files = self.complete_files()
            files.pop("lazyss_0.1.0_windows_arm64.zip")
            self.write_dist(dist, files)

            events = self.module.verify_dist(dist)

            self.assertEqual(events[0].level, "fail")
            self.assertIn("missing required archive", events[0].message)
            self.assertIn("windows/arm64", events[0].message)

    def test_rejects_checksum_mismatch(self):
        with tempfile.TemporaryDirectory() as tmp:
            dist = pathlib.Path(tmp) / "dist"
            self.write_dist(dist, self.complete_files())
            archive = dist / "lazyss_0.1.0_linux_amd64.tar.gz"
            archive.write_bytes(b"tampered")

            events = self.module.verify_dist(dist)

            self.assertEqual(events[0].level, "fail")
            self.assertIn("checksum mismatch", events[0].message)


if __name__ == "__main__":
    unittest.main()
