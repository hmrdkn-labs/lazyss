## Goal / Issue

## DDD Boundary Touched

- [ ] `internal/domain`
- [ ] `internal/ports`
- [ ] `internal/app`
- [ ] `internal/adapters/sshconfig`
- [ ] `internal/adapters/awsssm`
- [ ] `internal/adapters/statejson`
- [ ] `internal/tui`
- [ ] `internal/doctor`
- [ ] docs/release/CI only

## Tests Added or Updated

## Verification Output

```sh
gofmt -l .
go vet ./...
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -1
go build ./cmd/lazyss
make smoke-local
```

For release-affecting changes, also link the release-candidate workflow run.
If the release-candidate workflow is not selected by changed files, apply the
`release-candidate` label before merge.
Before requesting release approval, include `./scripts/release-readiness.sh`
output with `LAZYSS_LIVE_SMOKE_EVIDENCE` set.

## Secret/State Safety

- [ ] No private keys, AWS credentials, GitHub tokens, SSO cache data, or environment dumps are printed or stored.
- [ ] Any local state changes preserve `0600` permissions.
- [ ] Commands are executable plus argv, not shell strings.

## Release/Admin Actions

- [ ] This PR does not create tags, repository secrets, tap repositories, branch protection, or public assets.
- [ ] If it does, explicit owner approval is linked above.
