#!/usr/bin/env python3
import argparse
import datetime
import json
import os
import pathlib
import re
import subprocess
import sys
from collections import namedtuple


Event = namedtuple("Event", ["level", "message"])

SECRET_PATTERNS = [
    ("GitHub token", r"\b(ghp_[A-Za-z0-9_]{20,}|github_pat_[A-Za-z0-9_]+|gho_[A-Za-z0-9_]{20,})\b"),
    ("Authorization bearer token", r"(?i)\bAuthorization:\s*Bearer\s+[A-Za-z0-9_.-]+"),
    ("Homebrew token assignment", r"\bHOMEBREW_GITHUB_API_TOKEN\s*="),
    ("AWS credential field", r"(?i)\b(AWS_SECRET_ACCESS_KEY|AWS_SESSION_TOKEN|aws_secret_access_key|aws_session_token)\b"),
    ("private key", r"BEGIN (OPENSSH|RSA|EC|DSA) PRIVATE KEY"),
    ("private asset API URL", r"https://api\.github\.com/repos/[^/]+/[^/]+/releases/assets/[0-9]+"),
]


def read_json(path):
    return json.loads(pathlib.Path(path).read_text(encoding="utf-8"))


def validate_raw(raw, release_version, head):
    for label, pattern in SECRET_PATTERNS:
        if re.search(pattern, raw):
            return [Event("fail", f"private Homebrew evidence appears to contain credential material: {label}")]

    try:
        data = json.loads(raw)
    except json.JSONDecodeError as exc:
        return [Event("fail", f"private Homebrew evidence is not valid JSON: {exc.msg}")]

    return validate_payload(data, release_version, head)


def validate_file(path, release_version, head):
    try:
        raw = pathlib.Path(path).read_text(encoding="utf-8")
    except OSError as exc:
        return [Event("blocker", f"could not read private Homebrew evidence: {exc}")]
    return validate_raw(raw, release_version, head)


def validate_payload(data, release_version, head):
    if not isinstance(data, dict):
        return [Event("fail", "private Homebrew evidence must be a JSON object")]

    blockers = []

    def require(path_parts, expected=True):
        cursor = data
        for part in path_parts:
            if not isinstance(cursor, dict) or part not in cursor:
                blockers.append(f"{'.'.join(path_parts)} is missing")
                return None
            cursor = cursor[part]
        if expected is not None and cursor != expected:
            blockers.append(f"{'.'.join(path_parts)} must be {expected!r}")
        return cursor

    if data.get("version") != 1:
        blockers.append("version must be 1")
    if data.get("target_version") != release_version:
        blockers.append(f"target_version must be {release_version}")
    if data.get("commit") != head:
        blockers.append("commit must match current HEAD")

    checked_at = data.get("checked_at")
    if not isinstance(checked_at, str) or not checked_at:
        blockers.append("checked_at is missing")
    else:
        try:
            datetime.datetime.fromisoformat(checked_at.replace("Z", "+00:00"))
        except ValueError:
            blockers.append("checked_at must be ISO-8601")

    tap = data.get("tap")
    if not isinstance(tap, dict):
        blockers.append("tap evidence must be an object")
    else:
        require(["tap", "repo"], "hamardikan/homebrew-tap")
        require(["tap", "tap"], "hamardikan/tap")
        require(["tap", "cask"], "lazyss")
        require(["tap", "tapped"], True)

    install = data.get("install")
    if not isinstance(install, dict):
        blockers.append("install evidence must be an object")
    else:
        require(["install", "passed"], True)
        require(["install", "clean_homebrew_environment"], True)
        require(["install", "private_asset_downloaded"], True)
        require(["install", "token_env_name"], "HOMEBREW_GITHUB_API_TOKEN")
        require(["install", "installed_version"], release_version)

    runtime = data.get("runtime")
    if not isinstance(runtime, dict):
        blockers.append("runtime evidence must be an object")
    else:
        require(["runtime", "version_passed"], True)
        require(["runtime", "doctor_completed"], True)

    safety = data.get("safety")
    if not isinstance(safety, dict):
        blockers.append("safety evidence must be an object")
    else:
        for field in (
            "no_token_in_logs",
            "no_token_in_cask",
            "no_private_asset_url_recorded",
            "token_value_not_recorded",
        ):
            require(["safety", field], True)

    if blockers:
        return [Event("blocker", f"private Homebrew evidence invalid: {message}") for message in blockers]

    return [
        Event("ok", f"private Homebrew install evidence validated for {release_version} at {head[:12]}"),
        Event("ok", "private Homebrew cask downloaded a private release asset using HOMEBREW_GITHUB_API_TOKEN"),
        Event("ok", "private Homebrew evidence safety checks passed without recorded token material"),
    ]


def template_payload(target_version, commit, now):
    checked_at = now.astimezone(datetime.timezone.utc).isoformat().replace("+00:00", "Z")
    return {
        "version": 1,
        "target_version": target_version,
        "commit": commit,
        "checked_at": checked_at,
        "operator": "local-release-operator",
        "tap": {
            "repo": "hamardikan/homebrew-tap",
            "tap": "hamardikan/tap",
            "cask": "lazyss",
            "tapped": False,
        },
        "install": {
            "passed": False,
            "clean_homebrew_environment": False,
            "private_asset_downloaded": False,
            "token_env_name": "HOMEBREW_GITHUB_API_TOKEN",
            "installed_version": target_version,
        },
        "runtime": {
            "version_passed": False,
            "doctor_completed": False,
        },
        "safety": {
            "no_token_in_logs": False,
            "no_token_in_cask": False,
            "no_private_asset_url_recorded": False,
            "token_value_not_recorded": False,
        },
        "notes": (
            "Use non-secret pass/fail evidence only. Do not include GitHub token values, "
            "authorization headers, private release asset API URLs, env dumps, AWS credentials, "
            "SSH keys, or Homebrew debug output that contains token material."
        ),
    }


def write_template(path, target_version, commit, now=None, force=False):
    now = now or datetime.datetime.now(datetime.timezone.utc)
    path = pathlib.Path(path)
    flags = os.O_WRONLY | os.O_CREAT
    if force:
        flags |= os.O_TRUNC
    else:
        flags |= os.O_EXCL
    fd = os.open(path, flags, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as handle:
        json.dump(template_payload(target_version, commit, now), handle, indent=2)
        handle.write("\n")
    os.chmod(path, 0o600)


def default_commit():
    return subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip()


def print_events(events):
    for event in events:
        print(f"{event.level}\t{event.message}")


def exit_code(events):
    if any(event.level == "fail" for event in events):
        return 1
    if any(event.level == "blocker" for event in events):
        return 2
    return 0


def main(argv=None):
    parser = argparse.ArgumentParser(description="Create or validate LazySS private Homebrew install evidence.")
    subparsers = parser.add_subparsers(dest="command", required=True)

    template = subparsers.add_parser("template", help="write an editable private Homebrew evidence template")
    template.add_argument("--output", default="homebrew-private-evidence.json")
    template.add_argument("--target-version", default=os.environ.get("LAZYSS_RELEASE_VERSION", "v0.1.0"))
    template.add_argument("--commit", default=None)
    template.add_argument("--force", action="store_true")

    validate = subparsers.add_parser("validate", help="validate private Homebrew evidence")
    validate.add_argument("--file", required=True)
    validate.add_argument("--target-version", required=True)
    validate.add_argument("--commit", required=True)

    args = parser.parse_args(argv)
    if args.command == "template":
        commit = args.commit or default_commit()
        try:
            write_template(args.output, args.target_version, commit, force=args.force)
        except FileExistsError:
            print(f"fail\t{args.output} already exists; pass --force to overwrite", file=sys.stderr)
            return 1
        print(f"ok\twrote {args.output} with mode 0600")
        return 0

    events = validate_file(args.file, args.target_version, args.commit)
    print_events(events)
    return exit_code(events)


if __name__ == "__main__":
    raise SystemExit(main())
