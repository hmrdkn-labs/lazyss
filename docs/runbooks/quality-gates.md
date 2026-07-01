# LazySS Quality Gates

Run these from the repository root before committing release-quality changes.

```sh
gofmt -l .
go vet ./...
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -1
go build ./cmd/lazyss
```

The Make targets mirror the hosted pipeline stages:

```sh
make check             # local core gate: fmt, go mod tidy, vet, race tests, script tests, build
make fast-pr           # local mirror of fast CI, including smoke/lint/vuln when tools exist
make heavy-quality     # coverage, lint, and vulnerability scan
make release-snapshot  # goreleaser check plus snapshot artifact generation
make release-artifacts-verify # verify archives and checksums under DIST=dist
make release-preflight # read-only release readiness audit
make live-smoke-evidence-template # create ignored 0600 live smoke evidence draft
```

Run the safe local binary/TUI smoke when the change affects runtime behavior,
release packaging, or the release candidate checklist:

```sh
make smoke-local
```

Current enforced coverage baseline:

```txt
coverage.baseline: 57.7%
```

`make test` and hosted fast CI compare `go tool cover -func` total coverage
against `coverage.baseline`. Do not lower the baseline without explaining the
reason in the PR. When coverage improves intentionally, raise the baseline in
the same PR.

## Hosted Fast CI

Fast CI runs on pull requests and pushes to `main`. Branch protection should use
one stable aggregate check as the required merge gate:

- `ci-required`: fails when any component fast CI job fails

The component jobs remain separate for fast diagnosis:

- `format`: `gofmt -l .`
- `mod-tidy`: `go mod tidy` followed by a clean `go.mod`/`go.sum` diff check
- `vet`: `go vet ./...`
- `test`: race tests, coverage profile, coverage baseline check, coverage artifact
- `script-test`: Python tests for release helper scripts
- `build`: linux `go build ./cmd/lazyss`
- `smoke-local`: safe local binary/TUI smoke with `make smoke-local`
- `lint`: pinned `golangci-lint`
- `govulncheck`: Go vulnerability scan

Fast CI jobs run independently so format, module tidy, vet, lint,
vulnerability, build, test, script-test, and smoke failures are visible
separately. The `ci-required` job depends on all component jobs and is the only
check branch protection should require. Each Go job has a timeout and uses Go
module caching through `actions/setup-go`.

Fast CI cancels superseded runs for the same pull request or branch. For branch
protection, require `ci-required` instead of a broad workflow-level status or
implementation-detail component job names.

Validate the read-only branch protection state before requesting release
approval:

```sh
./scripts/branch-protection-readiness.sh
```

## Release Candidate Workflow

The release-candidate workflow has a lightweight `classify` job. The heavy
release proof jobs run when:

- the event is a relevant `main` push
- the workflow is started manually with `workflow_dispatch`
- a pull request changes release-relevant files such as workflows, GoReleaser
  config, lint config, coverage baseline, policy templates, release runbooks,
  `Makefile`, `cmd/`, `internal/`, Go modules, or `scripts/`
- a pull request has the `release-candidate` or `release` label

The release proof jobs are:

- linux/darwin/windows amd64/arm64 build matrix
- GoReleaser snapshot validation
- archive and checksum verification for the snapshot `dist/` directory
- short-retention upload of the `dist/` snapshot artifacts
- Homebrew readiness audit on macOS
- `release-candidate-required` aggregate status

The Homebrew readiness audit is allowed to report approval/external-state
blockers before tap approval. Local configuration failures still fail the
release-candidate workflow.

Download the `goreleaser-snapshot-<sha>` artifact from the release-candidate run
when reviewing archive names, checksums, or generated cask output before a real
tag.

Use the `release-candidate` label when a docs-only or policy-only PR still needs
the heavier release proof before merge.

## Release Candidate Gates

Before tagging `v0.1.0`, verify:

- `LAZYSS_LIVE_SMOKE_EVIDENCE=live-smoke-evidence.json ./scripts/release-readiness.sh` exits `0`
- `make smoke-local` passes on the release candidate checkout
- release-candidate workflow has passed for the release candidate commit
- `lazyss --version` prints the intended release version
- `lazyss doctor` runs without leaking credentials
- SSH inventory reads config without mutating it
- direct SSH launch works for one known host
- AWS degraded setup does not hide SSH inventory
- one AWS SSM-ready instance can be inventoried and launched
- state permissions and failed connection history remain correct

For release issues or audits that need machine-readable evidence, generate
structured reports without changing release state:

```sh
LAZYSS_LIVE_SMOKE_EVIDENCE=live-smoke-evidence.json \
LAZYSS_RELEASE_READINESS_JSON=release-readiness.json \
LAZYSS_RELEASE_READINESS_MARKDOWN=release-readiness.md \
./scripts/release-readiness.sh
```

Do not commit generated readiness reports or live smoke evidence files unless a
release issue explicitly requires an attached artifact. The root-level
`live-smoke-evidence.json`, `release-readiness.json`, and
`release-readiness.md` filenames are ignored locally to reduce accidental
commits.
