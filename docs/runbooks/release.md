# LazySS Release Runbook

Do not tag `v0.1.0` until every prerequisite below is verified.

## Prerequisites

- `main` is green in fast GitHub CI.
- The release-candidate workflow has passed for the release candidate commit.
- GoReleaser snapshot has passed.
- Homebrew private cask proof has passed or an approved fallback ADR exists.
- Branch protection readiness has passed.
- Real smoke tests have passed for one SSH host and one AWS SSM-ready instance.
- No private keys, AWS credentials, GitHub tokens, SSO cache data, or
  environment dumps appear in docs, generated casks, state files, or logs.

Run the read-only Homebrew readiness audit before requesting tap or release
approval:

```sh
./scripts/homebrew-readiness.sh
```

`exit 2` means the local config is ready but approval or external state is still
missing. Do not tag while this command reports blockers.

Run the full read-only release readiness audit before requesting `v0.1.0`
approval:

```sh
LAZYSS_LIVE_SMOKE_EVIDENCE=live-smoke-evidence.json ./scripts/release-readiness.sh
```

For a release issue, generate structured evidence:

```sh
LAZYSS_LIVE_SMOKE_EVIDENCE=live-smoke-evidence.json \
LAZYSS_RELEASE_READINESS_JSON=release-readiness.json \
LAZYSS_RELEASE_READINESS_MARKDOWN=release-readiness.md \
./scripts/release-readiness.sh
```

This audit checks the current branch, clean worktree, repo privacy, latest fast
CI, latest release-candidate workflow, branch protection, tag/release absence,
Homebrew readiness, local AWS SSM prerequisite tooling, and live smoke evidence.
It does not create repositories, secrets, branch protection, tags, releases, or
public assets.

Branch protection is validated by `./scripts/branch-protection-readiness.sh`.
That audit requires protected fast CI checks, pull request reviews, up-to-date
branches, and disabled force pushes/deletions.

Exit codes:

- `0`: release readiness prerequisites are satisfied.
- `1`: local release configuration or tool setup has a fixable failure.
- `2`: approval, external-state, or live-smoke blockers remain.

The `make release-readiness` target is a convenience wrapper for local human
use, but call the script directly when exact exit-code handling matters.

The JSON and Markdown report environment variables are optional. They write the
same check levels and messages that appear in the terminal output. Do not place
token values, credential dumps, SSH keys, AWS SSO cache data, or private release
asset URLs in these reports.

Live smoke proof must be a local JSON file referenced by
`LAZYSS_LIVE_SMOKE_EVIDENCE`. Use
`docs/runbooks/live-smoke-evidence.example.json` as the starting schema. The
readiness audit ignores legacy one-shot smoke environment flags for release
approval because they are not auditable.

## Local Gates

Run:

   ```sh
   gofmt -l .
   go vet ./...
   go test -race -coverprofile=coverage.out ./...
   go tool cover -func=coverage.out | tail -1
   go build ./cmd/lazyss
   ```

## Snapshot Release

Run:

```sh
goreleaser check
goreleaser release --clean --snapshot --skip=publish
```

If `goreleaser` is not installed locally, use the GitHub Actions snapshot job
from the release-candidate workflow and record the run URL in the release issue
or PR.

The release-candidate workflow can be forced before merge by applying the
`release-candidate` label to a pull request. Use this for release policy,
runbook, or packaging changes that do not otherwise touch release-relevant code
paths.

## Tag

Only after owner approval:

   ```sh
   git tag v0.1.0
   git push origin v0.1.0
   ```

Watch release CI:

```sh
gh run watch --repo hamardikan/lazyss
```

Confirm:

- GitHub Release `v0.1.0` exists.
- Archives exist for linux/darwin/windows amd64/arm64.
- `checksums.txt` exists.
- Homebrew cask is generated or published according to ADR 0002.
- `lazyss --version` prints `v0.1.0`.
