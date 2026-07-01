# LazySS Branch Protection Runbook

Branch protection is an owner-approved release gate. Do not enable or modify
GitHub branch protection without explicit approval.

## Target State

Protect `main` with:

- pull requests required before merge
- approving reviews are optional in solo-maintainer mode because GitHub does
  not count the pull request author as the required approving reviewer
- branch must be up to date before merge
- required fast CI check:
  - `ci-required`
- force pushes disabled
- branch deletion disabled

`ci-required` is a stable aggregate gate in the `CI` workflow. It fails if any
of the component jobs fail: `format`, `vet`, `test`, `script-test`, `build`,
`smoke-local`, `lint`, or `govulncheck`. Require the aggregate check in branch
protection so workflow internals can change without editing GitHub branch
protection every time a job is renamed or split.

Warnings, not hard release blockers for V1:

- administrators are not included in enforcement
- linear history is not required

Set `LAZYSS_REQUIRED_APPROVING_REVIEWS=1` only after a second trusted
collaborator exists. Until then, requiring one review deadlocks the protected
branch because the only maintainer cannot approve their own pull request.

## Read-Only Audit

Run:

```sh
./scripts/branch-protection-readiness.sh
```

Exit codes:

- `0`: branch protection matches the required LazySS release policy.
- `1`: local tooling or API parsing failed.
- `2`: branch protection is missing or incomplete and owner approval is needed.

The script does not enable, modify, or delete branch protection. It only reads
GitHub API state through `gh`.

## Owner-Approved Setup

After explicit owner approval, configure the policy in GitHub settings or with
`gh api`. Do not include token values in commands, docs, PRs, or logs.

After changing settings, rerun:

```sh
./scripts/branch-protection-readiness.sh
LAZYSS_LIVE_SMOKE_EVIDENCE=live-smoke-evidence.json ./scripts/release-readiness.sh
```
