# Contributing to LazySS

LazySS uses pragmatic DDD boundaries and strict verification. Keep changes close
to the package boundary they affect.

## Branch Flow

Use feature branches:

```sh
git switch -c feat/<short-scope>
```

Open a draft PR early for non-trivial work. Each PR should include:

- goal or issue link
- DDD boundary touched
- tests added or updated
- exact verification output
- secret/state safety notes

Use conventional commits:

```txt
test: define release gate behavior
feat: add goreleaser snapshot config
docs: add homebrew private asset runbook
ci: split quality gates
```

## DDD Boundaries

- `internal/domain`: pure types and rules
- `internal/ports`: interfaces owned by the app/domain layer
- `internal/app`: use cases and orchestration
- `internal/adapters/*`: external systems and persistence
- `internal/tui`: Bubble Tea cockpit over app use cases
- `internal/doctor`: local readiness checks

Do not move behavior across boundaries unless a failing test or architecture ADR
justifies it.

## Test Flow

Use TDD when behavior changes:

1. Write or update a focused failing test.
2. Run the focused package test and see it fail for the expected reason.
3. Implement the smallest production change.
4. Re-run the focused test.
5. Run the full gate before commit.

## Local Gates

```sh
gofmt -l .
go vet ./...
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -1
go build ./cmd/lazyss
make smoke-local
```

## Release Safety

Do not create release tags, tap repositories, repository secrets, branch
protection, or public release assets without explicit owner approval.

Never print private keys, AWS credentials, SSO cache data, GitHub tokens, or
environment dumps.
