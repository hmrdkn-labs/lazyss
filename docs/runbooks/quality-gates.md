# LazySS Quality Gates

Run these from the repository root before committing release-quality changes.

```sh
gofmt -l .
go vet ./...
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -1
go build ./cmd/lazyss
```

Current report-only coverage baseline:

```txt
total: (statements) 57.7%
```

Coverage is a signal for V1, not a hard percentage gate. Do not lower coverage
without explaining the reason in the PR.

## Hosted Gates

GitHub CI must pass:

- format check
- vet
- race tests with coverage artifact
- coverage summary
- pinned `golangci-lint`
- `govulncheck`
- linux/darwin/windows amd64/arm64 build matrix
- GoReleaser snapshot validation

## Release Candidate Gates

Before tagging `v0.1.0`, verify:

- `lazyss --version` prints the intended release version
- `lazyss doctor` runs without leaking credentials
- SSH inventory reads config without mutating it
- direct SSH launch works for one known host
- AWS degraded setup does not hide SSH inventory
- one AWS SSM-ready instance can be inventoried and launched
- state permissions and failed connection history remain correct
