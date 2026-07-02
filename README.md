# Lazy Secure Shell

`lazyss` is a terminal machine cockpit for direct SSH and AWS Systems Manager
Session Manager. It discovers machines from read-only SSH config and AWS
SSM/EC2 inventory, shows method-specific reachability, launches secure
sessions, and stores local operator memory such as pins, notes, checks, and
connection history.

## Scope

V1 supports:

- read-only `~/.ssh/config` inventory
- AWS SSM + EC2 inventory
- direct `ssh` sessions
- `aws ssm start-session` sessions
- manual health checks
- local JSON state under the OS user config directory

V1 deliberately does not edit SSH config, write ProxyCommand entries, run a
monitoring daemon, store secrets, or implement non-AWS cloud adapters.

## Usage

After installing `lazyss`:

```sh
lazyss
lazyss doctor
lazyss --source all
lazyss --source ssh --ssh-config ~/.ssh/config
lazyss --source aws --aws-profile prod --aws-region ap-southeast-1
```

## Install

### From a Local Checkout

```sh
make build
./bin/lazyss --version
```

### From GitHub Releases

After `v0.1.0`, download the archive for your OS and architecture from the
private GitHub release, verify it against `checksums.txt`, and put `lazyss` on
your `PATH`.

### Homebrew

After the private Homebrew tap is approved and published:

```sh
brew install --formula hamardikan/tap/lazyss
```

Private release assets require `HOMEBREW_GITHUB_API_TOKEN` in the operator
shell. Do not print the token value. The tap may also contain a cask, but the
formula is the primary CLI install path because unsigned cask binaries can be
blocked by macOS quarantine.

### Go Install

While the repository is private, `go install` is a developer path only. It
requires GitHub authentication and `GOPRIVATE` configuration:

```sh
GOPRIVATE=github.com/hamardikan/* go install github.com/hamardikan/lazyss/cmd/lazyss@latest
```

## Verification

```sh
gofmt -l .
go vet ./...
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -1
go build ./cmd/lazyss
make smoke-local
```

## Release Status

`v0.1.0` must not be tagged until local gates, fast hosted CI, the
release-candidate workflow, Homebrew readiness, and real SSH/AWS SSM smoke
tests pass. For the first release, token-backed Homebrew install proof is a
post-publish gate because the private package and release assets do not exist
before publishing.

Use the read-only readiness audit before requesting release approval:

```sh
LAZYSS_LIVE_SMOKE_EVIDENCE=live-smoke-evidence.json \
./scripts/release-readiness.sh
```

Release readiness can also emit JSON and Markdown evidence for a release issue:

```sh
LAZYSS_LIVE_SMOKE_EVIDENCE=live-smoke-evidence.json \
LAZYSS_RELEASE_READINESS_JSON=release-readiness.json \
LAZYSS_RELEASE_READINESS_MARKDOWN=release-readiness.md \
./scripts/release-readiness.sh
```

Create ignored local evidence drafts with:

```sh
make live-smoke-evidence-template
make homebrew-private-evidence-template
```

Before asking for owner approval on branch protection, tap creation, repository
secrets, local tap setup, or tagging, generate the ignored local handoff:

```sh
make release-approval-plan
```

Do not put secrets, private keys, token values, SSO cache data, environment
dumps, private release asset URLs, or full terminal logs in evidence files.
