# LazySS Release Runbook

Do not tag `v0.1.0` until every prerequisite below is verified.

## Prerequisites

- `main` is green in GitHub CI.
- GoReleaser snapshot has passed.
- Homebrew private cask proof has passed or an approved fallback ADR exists.
- Real smoke tests have passed for one SSH host and one AWS SSM-ready instance.
- No private keys, AWS credentials, GitHub tokens, SSO cache data, or
  environment dumps appear in docs, generated casks, state files, or logs.

Run the read-only Homebrew readiness audit before requesting tap or release
approval:

```sh
make homebrew-readiness
```

`exit 2` means the local config is ready but approval or external state is still
missing. Do not tag while this command reports blockers.

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
and record the run URL in the release issue or PR.

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
