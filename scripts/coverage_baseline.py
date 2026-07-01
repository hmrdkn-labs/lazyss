#!/usr/bin/env python3
import argparse
import pathlib
import re
import sys
from collections import namedtuple
from decimal import Decimal, InvalidOperation


Event = namedtuple("Event", ["level", "message"])
TOTAL_RE = re.compile(r"^total:\s+\(statements\)\s+(?P<percent>\d+(?:\.\d+)?)%$")


def read_text(path, label):
    try:
        return pathlib.Path(path).read_text(encoding="utf-8"), None
    except OSError as exc:
        return None, Event("fail", f"could not read {label}: {exc}")


def parse_baseline(text):
    for line in text.splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        try:
            return Decimal(line.rstrip("%"))
        except InvalidOperation:
            return None
    return None


def parse_total_coverage(text):
    for line in text.splitlines():
        match = TOTAL_RE.match(line.strip())
        if match:
            return Decimal(match.group("percent"))
    return None


def verify_coverage(summary_path, baseline_path):
    summary, err = read_text(summary_path, "coverage summary")
    if err:
        return [err]

    baseline_text, err = read_text(baseline_path, "coverage baseline")
    if err:
        return [err]

    baseline = parse_baseline(baseline_text)
    if baseline is None:
        return [Event("fail", "coverage baseline must contain a numeric percentage")]

    total = parse_total_coverage(summary)
    if total is None:
        return [Event("fail", "coverage summary is missing the total coverage line")]

    if total < baseline:
        return [Event("fail", f"coverage {total}% is below baseline {baseline}%")]

    return [Event("ok", f"coverage {total}% meets baseline {baseline}%")]


def print_events(events):
    for event in events:
        print(f"{event.level}\t{event.message}")


def exit_code(events):
    return 1 if any(event.level == "fail" for event in events) else 0


def main(argv=None):
    parser = argparse.ArgumentParser(description="Verify LazySS total test coverage against the tracked baseline.")
    subparsers = parser.add_subparsers(dest="command", required=True)
    verify = subparsers.add_parser("verify", help="verify a go tool cover -func summary")
    verify.add_argument("--summary", default="coverage.txt", help="path to go tool cover -func output")
    verify.add_argument("--baseline", default="coverage.baseline", help="path to the tracked coverage baseline")

    args = parser.parse_args(argv)
    events = verify_coverage(args.summary, args.baseline)
    print_events(events)
    return exit_code(events)


if __name__ == "__main__":
    raise SystemExit(main())
