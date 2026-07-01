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
make fast-pr           # local mirror of fast CI, including smoke, lint, and pinned govulncheck
make heavy-quality     # coverage, lint, and pinned govulncheck
make workflow-policy   # static policy checks for GitHub Actions workflow shape
make build-matrix      # local cross-platform compile matrix
make release-candidate-local # local release-candidate mirror for matrix, snapshot, and Homebrew readiness
make release-snapshot  # goreleaser check plus snapshot artifact generation
make release-artifacts-verify # verify archives, binaries, checksums, and generated cask under DIST=dist
make release-preflight # read-only release readiness audit
make live-smoke-evidence-template # create ignored 0600 live smoke evidence draft
make homebrew-private-evidence-template # create ignored 0600 private Homebrew proof draft
make branch-protection-plan # create ignored branch-protection approval handoff files
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

Current Go toolchain baseline:

```txt
go.mod: go 1.25.11
```

Keep the patch-level Go version current when `govulncheck` reports standard
library vulnerabilities. Hosted workflows use `actions/setup-go` with
`go-version-file: go.mod`, so this value controls both local and hosted
standard-library vulnerability posture.

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
- `govulncheck`: pinned direct `go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...`

Fast CI jobs run independently so format, module tidy, vet, lint,
vulnerability, build, test, script-test, and smoke failures are visible
separately. The `ci-required` job depends on all component jobs and is the only
check branch protection should require. Each Go job has a timeout and uses Go
module caching through `actions/setup-go`. The vulnerability scan intentionally
uses the Go tool directly instead of the `golang/govulncheck-action` wrapper so
the PR workflow has one fewer transitive action dependency.

Fast CI cancels superseded runs for the same pull request or branch. For branch
protection, require `ci-required` instead of a broad workflow-level status or
implementation-detail component job names.

Validate the read-only branch protection state before requesting release
approval:

```sh
./scripts/branch-protection-readiness.sh
```

If that audit reports branch protection as missing, generate the local approval
handoff without mutating GitHub:

```sh
make branch-protection-plan
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
- archive, binary-content, checksum, and generated private-cask verification for
  the snapshot `dist/` directory
- host-matching archive execution smoke using `lazyss --version`
- short-retention upload of the `dist/` snapshot artifacts
- Homebrew readiness audit on macOS
- `release-candidate-required` aggregate status

The classifier writes `should_run` and `reason` to the GitHub job summary so a
reviewer can tell why the heavier proof did or did not execute without opening
raw logs. Local workflow policy tests cover the classifier summary, job
timeouts, release tag-only behavior, read-only PR workflow permissions, and the
absence of publish secrets from PR workflows.

The Homebrew readiness audit is allowed to report approval/external-state
blockers before tap approval. Local configuration failures still fail the
release-candidate workflow.

Use the local mirror before requesting a release-candidate review when
GoReleaser is installed:

```sh
make release-candidate-local
```

This target runs the local cross-platform compile matrix, GoReleaser snapshot
validation, archive/cask verification, and Homebrew readiness with the same
approval/external-state blocker semantics used by hosted release-candidate CI.

Download the `goreleaser-snapshot-<sha>` artifact from the release-candidate run
when reviewing archive names, checksums, or generated cask output before a real
tag. The `release-artifacts-verify` gate checks that `homebrew/Casks/lazyss.rb`
uses the private GitHub download strategy and that its platform checksums match
the generated archives. It also unpacks every archive to verify the expected
`lazyss` or `lazyss.exe` binary is present and non-empty, with executable mode
set for tar archives. The hosted release-candidate workflow opts into
`--smoke-host-binary`, which extracts the archive matching the runner host and
runs `lazyss --version`.

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
