#!/usr/bin/env python3
import argparse
import pathlib
import re
from dataclasses import dataclass


JOB_RE = re.compile(r"^  ([A-Za-z0-9_-]+):\s*$")
SECRET_REF_RE = re.compile(r"\bsecrets\.")
WRITE_PERMISSION_RE = re.compile(r"^\s+[A-Za-z0-9_-]+:\s*write\s*$")


@dataclass(frozen=True)
class Job:
    name: str
    body: str


@dataclass(frozen=True)
class Workflow:
    name: str
    path: pathlib.Path
    text: str
    jobs: tuple[Job, ...]


def extract_jobs(text):
    lines = text.splitlines()
    in_jobs = False
    jobs = []
    current_name = None
    current_lines = []

    def flush_current():
        if current_name is not None:
            jobs.append(Job(current_name, "\n".join(current_lines)))

    for line in lines:
        if line == "jobs:":
            in_jobs = True
            continue
        if not in_jobs:
            continue
        if line and not line.startswith(" "):
            flush_current()
            break
        match = JOB_RE.match(line)
        if match:
            flush_current()
            current_name = match.group(1)
            current_lines = [line]
            continue
        if current_name is not None:
            current_lines.append(line)
    else:
        if in_jobs:
            flush_current()

    return tuple(jobs)


def load_workflows(workflows_dir):
    workflows = {}
    for path in sorted(pathlib.Path(workflows_dir).glob("*.yml")):
        text = path.read_text(encoding="utf-8")
        workflows[path.name] = Workflow(path.name, path, text, extract_jobs(text))
    return workflows


def jobs_missing_timeouts(workflows):
    missing = []
    for workflow in workflows.values():
        for job in workflow.jobs:
            if "timeout-minutes:" not in job.body:
                missing.append(f"{workflow.name}:{job.name}")
    return missing


def non_release_secret_or_write_violations(workflows):
    violations = []
    for workflow in workflows.values():
        if workflow.name == "release.yml":
            continue
        if SECRET_REF_RE.search(workflow.text):
            violations.append(f"{workflow.name}: uses secrets in non-release workflow")
        if "permissions: write-all" in workflow.text:
            violations.append(f"{workflow.name}: uses write-all permissions")
        for line in workflow.text.splitlines():
            if WRITE_PERMISSION_RE.match(line):
                violations.append(f"{workflow.name}: grants write permission with line `{line.strip()}`")
    return violations


def release_is_semver_tag_only(workflow):
    text = workflow.text
    return (
        'tags:' in text
        and '"v[0-9]+.[0-9]+.[0-9]+"' in text
        and "pull_request:" not in text
        and "workflow_dispatch:" not in text
        and 'branches: ["main"]' not in text
    )


def step_runs_before(workflow, earlier, later):
    earlier_index = workflow.text.find(earlier)
    later_index = workflow.text.find(later)
    return earlier_index != -1 and later_index != -1 and earlier_index < later_index


def _step_block(workflow, step_name):
    lines = workflow.text.splitlines()
    start = None
    start_indent = None
    for index, line in enumerate(lines):
        if f"name: {step_name}" in line:
            start = index
            start_indent = len(line) - len(line.lstrip(" "))
            break
    if start is None:
        return ""

    block = [lines[start]]
    for line in lines[start + 1 :]:
        indent = len(line) - len(line.lstrip(" "))
        if line.lstrip().startswith("- name:") and indent <= start_indent:
            break
        block.append(line)
    return "\n".join(block)


def step_writes_summary(workflow, step_name):
    return "GITHUB_STEP_SUMMARY" in _step_block(workflow, step_name)


def workflow_action_uses(workflows, action_prefix):
    matches = []
    needle = f"uses: {action_prefix}"
    for workflow in workflows.values():
        for line_number, line in enumerate(workflow.text.splitlines(), start=1):
            if needle in line:
                matches.append(f"{workflow.name}:{line_number}:{line.strip()}")
    return matches


def verify(workflows_dir):
    workflows = load_workflows(workflows_dir)
    failures = []
    failures.extend(jobs_missing_timeouts(workflows))
    failures.extend(non_release_secret_or_write_violations(workflows))
    if "release.yml" in workflows and not release_is_semver_tag_only(workflows["release.yml"]):
        failures.append("release.yml is not semver-tag-only")
    failures.extend(workflow_action_uses(workflows, "golang/govulncheck-action"))
    return failures


def main(argv=None):
    parser = argparse.ArgumentParser(description="Verify LazySS GitHub Actions workflow policy.")
    parser.add_argument("--workflows-dir", default=".github/workflows")
    args = parser.parse_args(argv)

    failures = verify(args.workflows_dir)
    for failure in failures:
        print(f"fail\t{failure}")
    if failures:
        return 1
    print("ok\tworkflow policy passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
