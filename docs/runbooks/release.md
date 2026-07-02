# LazySS Release Runbook

Do not tag `v0.1.0` until every prerequisite below is verified.

## Prerequisites

- `main` is green in fast GitHub CI.
- The release-candidate workflow has passed for the release candidate commit.
- GoReleaser snapshot has passed.
- Homebrew readiness has passed for the public tap and publishing secret.
- Branch protection readiness has passed.
- Real smoke tests have passed for one SSH host and one AWS SSM-ready instance.
- No private keys, AWS credentials, GitHub tokens, SSO cache data, or
  environment dumps appear in docs, generated formula files, state files, or
  logs.

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
LAZYSS_LIVE_SMOKE_EVIDENCE=live-smoke-evidence.json \
./scripts/release-readiness.sh
```

For a release issue, generate structured evidence:

```sh
LAZYSS_LIVE_SMOKE_EVIDENCE=live-smoke-evidence.json \
LAZYSS_RELEASE_READINESS_JSON=release-readiness.json \
LAZYSS_RELEASE_READINESS_MARKDOWN=release-readiness.md \
./scripts/release-readiness.sh
```

This audit checks the current branch, clean worktree, public repo visibility,
latest fast CI, latest release-candidate workflow, branch protection,
tag/release absence, Homebrew readiness, local AWS SSM prerequisite tooling, and
live smoke evidence. It does not create repositories, secrets, branch
protection, tags, releases, or public assets.

Before requesting approval for any release-blocking mutation, generate the
local handoff:

```sh
make release-approval-plan
```

Branch protection is validated by:

```sh
./scripts/branch-protection-readiness.sh
```

If branch protection is not configured yet, generate the local read-only
handoff before requesting owner approval:

```sh
make branch-protection-plan
```

Exit codes:

- `0`: release readiness prerequisites are satisfied.
- `1`: local release configuration or tool setup has a fixable failure.
- `2`: approval, external-state, or live-smoke blockers remain.

The JSON and Markdown report environment variables are optional. They write the
same check levels and messages that appear in terminal output. Do not place
token values, credential dumps, SSH keys, AWS SSO cache data, or credentialed
release asset URLs in these reports.

Live smoke proof must be a local JSON file referenced by
`LAZYSS_LIVE_SMOKE_EVIDENCE`. Use `make live-smoke-evidence-template` to create
an ignored `0600` draft for the current commit. Edit that file after live SSH
and AWS SSM smoke checks pass, then validate it with:

```sh
python3 scripts/live_smoke_evidence.py validate \
  --file live-smoke-evidence.json \
  --target-version v0.1.0 \
  --commit "$(git rev-parse HEAD)"
```

The tag-driven GitHub release workflow runs the same readiness audit before
GoReleaser publishes artifacts. In hosted release mode it sets
`LAZYSS_RELEASE_READINESS_MODE=tag`, verifies the tag points at `origin/main`,
and reads live smoke proof from the repository secret
`LAZYSS_LIVE_SMOKE_EVIDENCE_JSON`. That secret must contain the JSON evidence
object, not token material.

Hosted release readiness uses `LAZYSS_RELEASE_READINESS_GITHUB_TOKEN`, not the
default workflow `GITHUB_TOKEN`, because it must read branch protection,
workflow runs, repository state, repository secret names, and the Homebrew tap.
Create or rotate that token only after explicit owner approval. It needs read
access to `hmrdkn-labs/lazyss` and `hmrdkn-labs/homebrew-tap`.

Hosted release mode skips operator-machine runtime tool checks because the
GitHub runner is not the SSH/AWS SSM operator machine. Live SSH and AWS SSM
proof remains mandatory through the evidence file. Hosted release mode also
sets `LAZYSS_HOMEBREW_READINESS_MODE=hosted`, which skips local `brew tap`
state but still verifies tracked Homebrew release configuration and GitHub
state.

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

When GoReleaser is installed locally, run the local release-candidate mirror
first:

```sh
make release-candidate-local
```

This runs the cross-platform compile matrix, snapshot artifact verification,
and Homebrew readiness audit with the same approval/external-state blocker
handling as the hosted release-candidate workflow.

Run:

```sh
goreleaser check
goreleaser release --clean --snapshot --skip=publish
```

If `goreleaser` is not installed locally, use the GitHub Actions snapshot job
from the release-candidate workflow and record the run URL in the release issue
or PR.

The release-candidate workflow uploads `dist/` as
`goreleaser-snapshot-<sha>` with short retention. Use that artifact to review
archive names, checksums, and generated formula content before approving a tag.
After downloading it, verify the expected platform archives, binary contents,
checksums, and generated formula:

```sh
DIST=/path/to/downloaded/dist make release-artifacts-verify
```

The hosted release-candidate gate additionally runs the archived binary that
matches the GitHub runner host with `--version`.

## Tag

Only after owner approval:

```sh
git tag v0.1.0
git push origin v0.1.0
```

Watch release CI:

```sh
gh run watch --repo hmrdkn-labs/lazyss
```

Confirm:

- GitHub Release `v0.1.0` exists.
- Archives exist for linux/darwin/windows amd64/arm64.
- Archives contain the expected non-empty `lazyss` or `lazyss.exe` binary, with
  executable mode set for tar archives.
- The host-matching release-candidate archive can execute `lazyss --version`.
- `checksums.txt` exists.
- `DIST=/path/to/release-artifacts make release-artifacts-verify` passes,
  including generated formula verification.
- Homebrew formula is published according to ADR 0002.
- `brew install --formula hmrdkn-labs/tap/lazyss` works.
- `lazyss --version` prints the release version.
