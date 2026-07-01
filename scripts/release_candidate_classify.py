#!/usr/bin/env python3
import argparse
import json
import re
import sys
from collections import namedtuple


Decision = namedtuple("Decision", ["should_run", "reason"])

RELEASE_LABELS = {"release-candidate", "release"}
RELEASE_RELEVANT_RE = re.compile(
    r"^("
    r"\.github/workflows/(ci|release-candidate|release)\.yml"
    r"|\.github/(CODEOWNERS|dependabot\.yml|pull_request_template\.md)"
    r"|\.github/ISSUE_TEMPLATE/"
    r"|\.goreleaser\.yaml"
    r"|\.golangci\.yml"
    r"|coverage\.baseline"
    r"|Makefile"
    r"|cmd/"
    r"|internal/"
    r"|go\.(mod|sum)"
    r"|scripts/"
    r"|docs/runbooks/(quality-gates|release|sdlc|homebrew|smoke|security)\.md"
    r")"
)


def classify(event_name, labels, changed_files):
    if event_name != "pull_request":
        return Decision(True, f"{event_name} event")

    if any(label in RELEASE_LABELS for label in labels):
        return Decision(True, "release label")

    if any(RELEASE_RELEVANT_RE.search(path) for path in changed_files):
        return Decision(True, "release-relevant files")

    return Decision(False, "not release-relevant; add release-candidate label to force")


def main(argv=None):
    parser = argparse.ArgumentParser(description="Classify whether LazySS release-candidate gates should run.")
    parser.add_argument("--event-name", required=True)
    parser.add_argument("--labels-json", default="[]")
    parser.add_argument("--changed-files", default="-", help="newline-delimited path file, or - for stdin")
    args = parser.parse_args(argv)

    try:
        labels = json.loads(args.labels_json)
    except json.JSONDecodeError as exc:
        print(f"invalid labels json: {exc}", file=sys.stderr)
        return 1

    if args.changed_files == "-":
        changed_files = sys.stdin.read().splitlines()
    else:
        with open(args.changed_files, encoding="utf-8") as handle:
            changed_files = handle.read().splitlines()

    decision = classify(args.event_name, labels, changed_files)
    print(f"should_run={str(decision.should_run).lower()}")
    print(f"reason={decision.reason}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
