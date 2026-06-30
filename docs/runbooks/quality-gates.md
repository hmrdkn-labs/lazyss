# LazySS Quality Gates

Run these from the repository root before committing release-quality changes.

```sh
gofmt -l .
go vet ./...
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -1
go build ./cmd/lazyss
```

Run the safe local binary/TUI smoke when the change affects runtime behavior,
release packaging, or the release candidate checklist:

```sh
make smoke-local
```

Current report-only coverage baseline:

```txt
total: (statements) 57.7%
```

Coverage is a signal for V1, not a hard percentage gate. Do not lower coverage
without explaining the reason in the PR.

## Hosted Fast CI

Fast CI runs on pull requests and pushes to `main`. Branch protection should use
these checks as the required merge gate:

- format check
- vet
- race tests with coverage artifact
- coverage summary
- linux `go build ./cmd/lazyss`
- safe local binary/TUI smoke with `make smoke-local`
- pinned `golangci-lint`
- `govulncheck`

Fast CI cancels superseded runs for the same pull request or branch.

## Release Candidate Workflow

The release-candidate workflow runs on release-automation pull requests,
relevant `main` pushes, and manual `workflow_dispatch`. It is the heavier
release proof gate:

- linux/darwin/windows amd64/arm64 build matrix
- GoReleaser snapshot validation
- Homebrew readiness audit on macOS

The Homebrew readiness audit is allowed to report approval/external-state
blockers before tap approval. Local configuration failures still fail the
release-candidate workflow.

## Release Candidate Gates

Before tagging `v0.1.0`, verify:

- `make smoke-local` passes on the release candidate checkout
- release-candidate workflow has passed for the release candidate commit
- `lazyss --version` prints the intended release version
- `lazyss doctor` runs without leaking credentials
- SSH inventory reads config without mutating it
- direct SSH launch works for one known host
- AWS degraded setup does not hide SSH inventory
- one AWS SSM-ready instance can be inventoried and launched
- state permissions and failed connection history remain correct
