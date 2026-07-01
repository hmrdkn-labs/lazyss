#!/usr/bin/env python3
import argparse
import json
import pathlib
import re
import sys


SECRET_RE = re.compile(
    r"ghp_[A-Za-z0-9_]+|github_pat_[A-Za-z0-9_]+|AWS_SECRET_ACCESS_KEY|AWS_SESSION_TOKEN|BEGIN (OPENSSH|RSA|EC|DSA) PRIVATE KEY"
)


def assert_no_credential_like_values(values):
    for value in values:
        if SECRET_RE.search(value):
            raise ValueError("credential-like value is not allowed in branch protection plan inputs")


def branch_protection_payload(required_checks, approving_review_count):
    return {
        "required_status_checks": {
            "strict": True,
            "contexts": required_checks,
        },
        "enforce_admins": True,
        "required_pull_request_reviews": {
            "dismiss_stale_reviews": True,
            "require_code_owner_reviews": False,
            "required_approving_review_count": approving_review_count,
            "require_last_push_approval": False,
        },
        "restrictions": None,
        "required_linear_history": True,
        "allow_force_pushes": False,
        "allow_deletions": False,
        "block_creations": False,
        "required_conversation_resolution": True,
        "lock_branch": False,
        "allow_fork_syncing": False,
    }


def markdown_plan(repo, branch, required_checks, approving_review_count, json_output):
    checks = ", ".join(required_checks)
    review_line = (
        f"Require {approving_review_count} approving review(s)."
        if approving_review_count > 0
        else "Require pull requests, but do not require approving reviews in solo-maintainer mode."
    )
    return f"""# LazySS Branch Protection Plan

Repository: `{repo}`
Branch: `{branch}`
Required checks: `{checks}`
Approving reviews: `{approving_review_count}`

This is a review artifact only. Applying it requires explicit owner approval.
The generator does not call GitHub APIs, create rules, or mutate branch
protection.

## Target Policy

- Require pull requests before merge.
- Require status checks to pass before merge.
- Require branches to be up to date before merge.
- Require `{checks}` as the stable protected check contract.
- {review_line}
- Dismiss stale approvals after new commits.
- Require conversation resolution.
- Require linear history.
- Include administrators.
- Disable force pushes.
- Disable branch deletion.

## Apply After Approval

```sh
gh api --method PUT repos/{repo}/branches/{branch}/protection \\
  --input {json_output}
```

## Verify

```sh
./scripts/branch-protection-readiness.sh
```

Expected verification result after the approved apply step: exit `0`.
"""


def write_text(path, text):
    destination = pathlib.Path(path)
    destination.parent.mkdir(parents=True, exist_ok=True)
    destination.write_text(text, encoding="utf-8")


def main(argv=None):
    parser = argparse.ArgumentParser(description="Generate a read-only LazySS branch protection setup plan.")
    parser.add_argument("--repo", default="hamardikan/lazyss")
    parser.add_argument("--branch", default="main")
    parser.add_argument("--required-check", action="append", default=None)
    parser.add_argument("--approving-review-count", type=int, default=0)
    parser.add_argument("--json-output", default="branch-protection.json")
    parser.add_argument("--markdown-output", default="branch-protection.md")
    args = parser.parse_args(argv)

    try:
        required_checks = args.required_check or ["ci-required"]
        assert_no_credential_like_values([args.repo, args.branch, *required_checks])
        if args.approving_review_count < 0:
            raise ValueError("approving review count must not be negative")
    except ValueError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2

    payload = branch_protection_payload(required_checks, args.approving_review_count)
    json_text = json.dumps(payload, indent=2, sort_keys=True) + "\n"
    markdown_text = markdown_plan(
        args.repo,
        args.branch,
        required_checks,
        args.approving_review_count,
        args.json_output,
    )

    write_text(args.json_output, json_text)
    write_text(args.markdown_output, markdown_text)
    print(f"wrote {args.json_output}")
    print(f"wrote {args.markdown_output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
