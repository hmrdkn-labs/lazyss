#!/usr/bin/env python3
import argparse
import hashlib
import platform
import pathlib
import re
import subprocess
import sys
import tarfile
import tempfile
import zipfile
from collections import namedtuple


Event = namedtuple("Event", ["level", "message"])

REQUIRED_TARGETS = (
    ("darwin", "amd64", ".tar.gz"),
    ("darwin", "arm64", ".tar.gz"),
    ("linux", "amd64", ".tar.gz"),
    ("linux", "arm64", ".tar.gz"),
    ("windows", "amd64", ".zip"),
    ("windows", "arm64", ".zip"),
)

ARCHIVE_RE = re.compile(r"^lazyss_.+_(?P<goos>darwin|linux|windows)_(?P<goarch>amd64|arm64)(?P<ext>\.tar\.gz|\.zip)$")
CASK_PATH = pathlib.Path("homebrew") / "Casks" / "lazyss.rb"
CASK_PRIVATE_STRATEGY_FRAGMENTS = (
    'cask "lazyss"',
    'class GitHubPrivateRepositoryReleaseDownloadStrategy < CurlDownloadStrategy',
    'ENV["HOMEBREW_GITHUB_API_TOKEN"]',
    'Authorization: Bearer #{@github_token}',
    'https://api.github.com/repos/#{@owner}/#{@repo}/releases/assets/#{asset_id}',
    "using: GitHubPrivateRepositoryReleaseDownloadStrategy",
    'verified: "github.com/hamardikan/lazyss/"',
    'binary "lazyss"',
)
CASK_SECRET_RE = re.compile(
    r"ghp_[A-Za-z0-9_]+|github_pat_[A-Za-z0-9_]+|AWS_SECRET_ACCESS_KEY|AWS_SESSION_TOKEN|BEGIN (OPENSSH|RSA|EC|DSA) PRIVATE KEY"
)


def read_checksums(path):
    checksums = {}
    try:
        lines = pathlib.Path(path).read_text(encoding="utf-8").splitlines()
    except OSError as exc:
        return None, [Event("fail", f"could not read checksums.txt: {exc}")]

    for line in lines:
        line = line.strip()
        if not line:
            continue
        parts = line.split()
        if len(parts) != 2:
            return None, [Event("fail", f"invalid checksums.txt line: {line}")]
        digest, filename = parts
        if not re.fullmatch(r"[0-9a-fA-F]{64}", digest):
            return None, [Event("fail", f"invalid sha256 digest for {filename}")]
        checksums[filename] = digest.lower()
    return checksums, []


def sha256(path):
    digest = hashlib.sha256()
    with pathlib.Path(path).open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def archive_targets(dist):
    found = {}
    for path in pathlib.Path(dist).iterdir():
        if not path.is_file():
            continue
        match = ARCHIVE_RE.match(path.name)
        if not match:
            continue
        target = (match.group("goos"), match.group("goarch"), match.group("ext"))
        found[target] = path
    return found


def verify_cask(dist, found, checksums):
    path = pathlib.Path(dist) / CASK_PATH
    try:
        text = path.read_text(encoding="utf-8")
    except OSError as exc:
        return [Event("fail", f"generated cask is missing or unreadable: {exc}")]

    missing = [fragment for fragment in CASK_PRIVATE_STRATEGY_FRAGMENTS if fragment not in text]
    if missing:
        return [Event("fail", f"generated cask is missing private download strategy fragment: {missing[0]}")]

    if CASK_SECRET_RE.search(text):
        return [Event("fail", "generated cask contains credential material")]

    if "#{@github_token}@" in text:
        return [Event("fail", "generated cask must not construct token-bearing URLs")]

    for goos, goarch, ext in REQUIRED_TARGETS:
        if goos == "windows":
            continue
        archive = found[(goos, goarch, ext)]
        expected = checksums[archive.name]
        if expected not in text:
            return [Event("fail", f"missing cask checksum for {archive.name}")]
        archive_suffix = f"_{goos}_{goarch}{ext}"
        if archive_suffix not in text:
            return [Event("fail", f"missing cask url for {goos}/{goarch}")]

    return [Event("ok", "generated cask uses private download strategy with matching archive checksums")]


def verify_tar_binary(path, goos, goarch):
    try:
        with tarfile.open(path, mode="r:gz") as archive:
            member = next((item for item in archive.getmembers() if item.isfile() and item.name == "lazyss"), None)
    except (OSError, tarfile.TarError) as exc:
        return [Event("fail", f"could not inspect {goos}/{goarch} tar archive: {exc}")]

    if member is None:
        return [Event("fail", f"missing lazyss binary in {goos}/{goarch} archive")]
    if member.size <= 0:
        return [Event("fail", f"lazyss binary is empty in {goos}/{goarch} archive")]
    if member.mode & 0o111 == 0:
        return [Event("fail", f"lazyss binary is not executable in {goos}/{goarch} archive")]
    return []


def verify_zip_binary(path, goos, goarch):
    try:
        with zipfile.ZipFile(path) as archive:
            try:
                info = archive.getinfo("lazyss.exe")
            except KeyError:
                return [Event("fail", f"missing lazyss.exe binary in {goos}/{goarch} archive")]
    except (OSError, zipfile.BadZipFile) as exc:
        return [Event("fail", f"could not inspect {goos}/{goarch} zip archive: {exc}")]

    if info.file_size <= 0:
        return [Event("fail", f"lazyss.exe binary is empty in {goos}/{goarch} archive")]
    return []


def verify_archive_contents(found):
    for goos, goarch, ext in REQUIRED_TARGETS:
        path = found[(goos, goarch, ext)]
        if ext == ".zip":
            errors = verify_zip_binary(path, goos, goarch)
        else:
            errors = verify_tar_binary(path, goos, goarch)
        if errors:
            return errors
    return [Event("ok", "archives contain expected binaries with installable permissions")]


def host_target():
    if sys.platform.startswith("linux"):
        goos = "linux"
    elif sys.platform == "darwin":
        goos = "darwin"
    elif sys.platform.startswith("win"):
        goos = "windows"
    else:
        return None

    machine = platform.machine().lower()
    if machine in ("x86_64", "amd64"):
        goarch = "amd64"
    elif machine in ("arm64", "aarch64"):
        goarch = "arm64"
    else:
        return None
    return goos, goarch


def write_tar_binary(path, destination):
    with tarfile.open(path, mode="r:gz") as archive:
        member = next((item for item in archive.getmembers() if item.isfile() and item.name == "lazyss"), None)
        if member is None:
            raise FileNotFoundError("lazyss")
        extracted = archive.extractfile(member)
        if extracted is None:
            raise FileNotFoundError("lazyss")
        destination.write_bytes(extracted.read())
        destination.chmod(member.mode | 0o700)


def write_zip_binary(path, destination):
    with zipfile.ZipFile(path) as archive:
        destination.write_bytes(archive.read("lazyss.exe"))
        destination.chmod(0o700)


def verify_host_binary(found):
    target = host_target()
    if target is None:
        return [Event("fail", "host platform is unsupported for archive binary smoke")]

    goos, goarch = target
    ext = ".zip" if goos == "windows" else ".tar.gz"
    archive_path = found.get((goos, goarch, ext))
    if archive_path is None:
        return [Event("fail", f"missing host archive for {goos}/{goarch}")]

    with tempfile.TemporaryDirectory() as tmp:
        binary_name = "lazyss.exe" if goos == "windows" else "lazyss"
        binary_path = pathlib.Path(tmp) / binary_name
        try:
            if ext == ".zip":
                write_zip_binary(archive_path, binary_path)
            else:
                write_tar_binary(archive_path, binary_path)
            result = subprocess.run(
                [str(binary_path), "--version"],
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True,
                timeout=10,
                check=False,
            )
        except (OSError, subprocess.SubprocessError, tarfile.TarError, zipfile.BadZipFile, KeyError) as exc:
            return [Event("fail", f"host archive binary failed --version smoke for {goos}/{goarch}: {exc}")]

    if result.returncode != 0:
        return [
            Event(
                "fail",
                f"host archive binary failed --version smoke for {goos}/{goarch}: exit {result.returncode}",
            )
        ]
    if not result.stdout.startswith("lazyss "):
        return [Event("fail", f"host archive binary returned unexpected --version output for {goos}/{goarch}")]
    return [Event("ok", f"host archive binary responds to --version for {goos}/{goarch}")]


def verify_dist(dist, smoke_host_binary=False):
    dist = pathlib.Path(dist)
    if not dist.is_dir():
        return [Event("fail", f"dist directory does not exist: {dist}")]

    checksums, errors = read_checksums(dist / "checksums.txt")
    if errors:
        return errors

    found = archive_targets(dist)
    for target in REQUIRED_TARGETS:
        if target not in found:
            goos, goarch, _ = target
            return [Event("fail", f"missing required archive for {goos}/{goarch}")]

    for target in REQUIRED_TARGETS:
        path = found[target]
        expected = checksums.get(path.name)
        if expected is None:
            return [Event("fail", f"missing checksum for {path.name}")]
        actual = sha256(path)
        if actual != expected:
            return [Event("fail", f"checksum mismatch for {path.name}")]

    cask_events = verify_cask(dist, found, checksums)
    if any(event.level == "fail" for event in cask_events):
        return cask_events

    content_events = verify_archive_contents(found)
    if any(event.level == "fail" for event in content_events):
        return content_events

    smoke_events = []
    if smoke_host_binary:
        smoke_events = verify_host_binary(found)
        if any(event.level == "fail" for event in smoke_events):
            return smoke_events

    return [
        Event("ok", "all required archives are present"),
        Event("ok", "checksums.txt includes every required archive"),
        Event("ok", "archive checksums match checksums.txt"),
        *cask_events,
        *content_events,
        *smoke_events,
    ]


def print_events(events):
    for event in events:
        print(f"{event.level}\t{event.message}")


def exit_code(events):
    return 1 if any(event.level == "fail" for event in events) else 0


def main(argv=None):
    parser = argparse.ArgumentParser(description="Verify LazySS release artifact archives and checksums.")
    subparsers = parser.add_subparsers(dest="command", required=True)
    verify = subparsers.add_parser("verify", help="verify a GoReleaser dist directory")
    verify.add_argument("--dist", default="dist")
    verify.add_argument(
        "--smoke-host-binary",
        action="store_true",
        help="extract the archive matching the current host and run lazyss --version",
    )

    args = parser.parse_args(argv)
    events = verify_dist(args.dist, smoke_host_binary=args.smoke_host_binary)
    print_events(events)
    return exit_code(events)


if __name__ == "__main__":
    raise SystemExit(main())
