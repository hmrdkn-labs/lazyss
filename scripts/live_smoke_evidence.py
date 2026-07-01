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
    ("AWS access key id", r"\b(AKIA|ASIA)[0-9A-Z]{16}\b"),
    (
        "AWS secret/session token field",
        r"(?i)\b(aws_secret_access_key|aws_session_token|AWS_SECRET_ACCESS_KEY|AWS_SESSION_TOKEN)\b",
    ),
    ("GitHub token", r"\b(ghp_[A-Za-z0-9_]{20,}|github_pat_[A-Za-z0-9_]+)\b"),
    ("private key", r"BEGIN (OPENSSH|RSA|EC|DSA) PRIVATE KEY"),
]


def read_json(path):
    return json.loads(pathlib.Path(path).read_text(encoding="utf-8"))


def validate_raw(raw, release_version, head):
    for label, pattern in SECRET_PATTERNS:
        if re.search(pattern, raw):
            return [Event("fail", f"live smoke evidence appears to contain credential material: {label}")]

    try:
        data = json.loads(raw)
    except json.JSONDecodeError as exc:
        return [Event("fail", f"live smoke evidence is not valid JSON: {exc.msg}")]

    return validate_payload(data, release_version, head)


def validate_file(path, release_version, head):
    try:
        raw = pathlib.Path(path).read_text(encoding="utf-8")
    except OSError as exc:
        return [Event("blocker", f"could not read live smoke evidence: {exc}")]
    return validate_raw(raw, release_version, head)


def validate_payload(data, release_version, head):
    if not isinstance(data, dict):
        return [Event("fail", "live smoke evidence must be a JSON object")]

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

    ssh = data.get("ssh")
    if not isinstance(ssh, dict):
        blockers.append("ssh evidence must be an object")
    else:
        require(["ssh", "passed"], True)
        require(["ssh", "config_mutated"], False)
        if not ssh.get("host_label"):
            blockers.append("ssh.host_label is missing")

    aws_ssm = data.get("aws_ssm")
    if not isinstance(aws_ssm, dict):
        blockers.append("aws_ssm evidence must be an object")
    else:
        for field in (
            "passed",
            "doctor_passed",
            "inventory_passed",
            "session_launch_passed",
            "degraded_ssh_preserved",
        ):
            require(["aws_ssm", field], True)
        if not aws_ssm.get("region"):
            blockers.append("aws_ssm.region is missing")
        if not aws_ssm.get("target_label"):
            blockers.append("aws_ssm.target_label is missing")

    safety = data.get("safety")
    if not isinstance(safety, dict):
        blockers.append("safety evidence must be an object")
    else:
        for field in ("no_secrets_observed", "state_mode_0600", "failed_connection_preserved_last_success"):
            require(["safety", field], True)

    if blockers:
        return [Event("blocker", f"live smoke evidence invalid: {message}") for message in blockers]

    return [
        Event("ok", f"live smoke evidence validated for {release_version} at {head[:12]}"),
        Event("ok", f"live SSH smoke passed for label {ssh.get('host_label')}"),
        Event("ok", f"live AWS SSM smoke passed for label {aws_ssm.get('target_label')} in {aws_ssm.get('region')}"),
        Event("ok", "live smoke safety checks passed without recorded secrets"),
    ]


def template_payload(target_version, commit, now):
    checked_at = now.astimezone(datetime.timezone.utc).isoformat().replace("+00:00", "Z")
    return {
        "version": 1,
        "target_version": target_version,
        "commit": commit,
        "checked_at": checked_at,
        "operator": "local-release-operator",
        "ssh": {
            "passed": False,
            "host_label": "fill-non-secret-ssh-label",
            "config_mutated": True,
            "health_checked": False,
            "session_launch_passed": False,
        },
        "aws_ssm": {
            "passed": False,
            "profile_label": "fill-non-secret-profile-label",
            "region": "fill-region",
            "target_label": "fill-non-secret-ssm-label",
            "doctor_passed": False,
            "inventory_passed": False,
            "session_launch_passed": False,
            "degraded_ssh_preserved": False,
        },
        "safety": {
            "no_secrets_observed": False,
            "state_mode_0600": False,
            "failed_connection_preserved_last_success": False,
        },
        "notes": (
            "Use non-secret labels only. Do not include host passwords, private keys, AWS credentials, "
            "SSO cache data, GitHub tokens, env dumps, or private release asset URLs."
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
    parser = argparse.ArgumentParser(description="Create or validate LazySS live smoke evidence.")
    subparsers = parser.add_subparsers(dest="command", required=True)

    template = subparsers.add_parser("template", help="write an editable live smoke evidence template")
    template.add_argument("--output", default="live-smoke-evidence.json")
    template.add_argument("--target-version", default=os.environ.get("LAZYSS_RELEASE_VERSION", "v0.1.0"))
    template.add_argument("--commit", default=None)
    template.add_argument("--force", action="store_true")

    validate = subparsers.add_parser("validate", help="validate live smoke evidence")
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
