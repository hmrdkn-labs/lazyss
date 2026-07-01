#!/usr/bin/env python3
import argparse
import os
import pathlib
import re
import sys


SECRET_RE = re.compile(
    r"ghp_[A-Za-z0-9_]+|github_pat_[A-Za-z0-9_]+|gho_[A-Za-z0-9_]+|"
    r"(?i:Authorization:\s*Bearer\s+[A-Za-z0-9_.-]+)|"
    r"HOMEBREW_GITHUB_API_TOKEN\s*=|AWS_SECRET_ACCESS_KEY|AWS_SESSION_TOKEN|"
    r"BEGIN (OPENSSH|RSA|EC|DSA) PRIVATE KEY"
)


def assert_no_credential_like_values(values):
    for value in values:
        if SECRET_RE.search(value):
            raise ValueError("credential-like value is not allowed in release approval plan inputs")


def markdown_plan(repo, tap_repo, tap, target_version):
    return f"""# LazySS Release Approval Plan

Repository: `{repo}`
Tap repository: `{tap_repo}`
Tap: `{tap}`
Target version: `{target_version}`

This generator is local and read-only. It writes this review artifact only and
does not create repositories, secrets, branch protection, tags, releases, or public assets.

## Mutation Policy

- Do not paste token values, AWS credentials, SSH keys, SSO cache data,
  authorization headers, private release asset URLs, or environment dumps into
  this file, issue comments, PR comments, logs, or generated evidence.
- Every command below that mutates GitHub, Homebrew, branch protection, local
  taps, tags, releases, or installed software requires explicit owner approval.
- Keep `lazyssh/` and `lazyssm/` ignored reference directories untouched.

## Approval Actions

1. Confirm operator-machine prerequisites.

   `session-manager-plugin` must be installed on the operator machine before
   the AWS SSM live smoke proof can pass. Use `lazyss doctor` and
   `./scripts/release-readiness.sh` to verify it after the owner-approved
   install.

2. Enable branch protection after reviewing the local handoff.

   ```sh
   make branch-protection-plan
   ```

   Review `branch-protection.md` and `branch-protection.json`. After explicit
   owner approval, apply the generated command from `branch-protection.md`, then
   verify:

   ```sh
   ./scripts/branch-protection-readiness.sh
   ```

3. Create or confirm the private Homebrew tap.

   Target repository: `{tap_repo}`
   Target local tap: `{tap}`

   The tap should stay private while `{repo}` remains private. Do not make
   release assets public unless that is an explicit release decision.

4. Add required repository secrets after owner approval.

   Secret names required by the release path:

   - `HOMEBREW_TAP_GITHUB_TOKEN`
   - `LAZYSS_RELEASE_READINESS_GITHUB_TOKEN`
   - `LAZYSS_LIVE_SMOKE_EVIDENCE_JSON`
   - `LAZYSS_HOMEBREW_PRIVATE_EVIDENCE_JSON`

   Store only the approved values in GitHub Secrets. Do not record secret values
   in this plan. The two evidence JSON secrets must contain redacted evidence
   objects only, not token material.

5. Tap the private Homebrew repository locally after it exists.

   ```sh
   brew tap {tap}
   ```

6. Capture live smoke proof.

   ```sh
   make live-smoke-evidence-template
   ```

   Fill `live-smoke-evidence.json` only after real SSH and AWS SSM smoke checks
   pass. Keep labels non-secret and validate the file before using it:

   ```sh
   python3 scripts/live_smoke_evidence.py validate \\
     --file live-smoke-evidence.json \\
     --target-version {target_version} \\
     --commit "$(git rev-parse HEAD)"
   ```

7. Capture private Homebrew install proof.

   ```sh
   make homebrew-private-evidence-template
   ```

   Fill `homebrew-private-evidence.json` only after a token-backed private cask
   install succeeds. Keep the token in the operator environment and out of
   evidence:

   ```sh
   brew install --cask {tap}/lazyss
   python3 scripts/homebrew_private_evidence.py validate \\
     --file homebrew-private-evidence.json \\
     --target-version {target_version} \\
     --commit "$(git rev-parse HEAD)"
   ```

## Final Readiness Command

Run the full release readiness audit from `main` with a clean worktree:

```sh
LAZYSS_LIVE_SMOKE_EVIDENCE=live-smoke-evidence.json \\
LAZYSS_HOMEBREW_PRIVATE_EVIDENCE=homebrew-private-evidence.json \\
LAZYSS_RELEASE_READINESS_JSON=release-readiness.json \\
LAZYSS_RELEASE_READINESS_MARKDOWN=release-readiness.md \\
./scripts/release-readiness.sh
```

Expected result before tagging: exit `0`.

## Tag After Green Readiness

Only after the readiness audit exits `0` and the owner approves the release:

```sh
git tag {target_version}
git push origin {target_version}
```

Watch the release workflow and verify the published artifacts through the
release runbook before using the release as the quality baseline.
"""


def write_text(path, text):
    destination = pathlib.Path(path)
    destination.parent.mkdir(parents=True, exist_ok=True)
    destination.write_text(text, encoding="utf-8")


def main(argv=None):
    parser = argparse.ArgumentParser(description="Generate a read-only LazySS release approval handoff.")
    parser.add_argument("--repo", default="hamardikan/lazyss")
    parser.add_argument("--tap-repo", default="hamardikan/homebrew-tap")
    parser.add_argument("--tap", default="hamardikan/tap")
    parser.add_argument("--target-version", default=os.environ.get("LAZYSS_RELEASE_VERSION", "v0.1.0"))
    parser.add_argument("--markdown-output", default="release-approval.md")
    args = parser.parse_args(argv)

    try:
        assert_no_credential_like_values([args.repo, args.tap_repo, args.tap, args.target_version])
    except ValueError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2

    write_text(args.markdown_output, markdown_plan(args.repo, args.tap_repo, args.tap, args.target_version))
    print(f"wrote {args.markdown_output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
