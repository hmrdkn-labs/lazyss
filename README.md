# Lazy Secure Shell

```txt
 _                     ____ ____
| |    __ _ _____   _ / ___/ ___|
| |   / _` |_  / | | |\___ \___ \
| |__| (_| |/ /| |_| | ___) |__) |
|_____\__,_/___|\__, ||____/____/
                |___/
Lazy Secure Shell
SSH + SSM cockpit
```

`lazyss` is a terminal machine cockpit for direct SSH and AWS Systems Manager
Session Manager. It discovers machines from read-only SSH config and AWS
SSM/EC2 inventory, shows method-specific reachability, launches secure
sessions, and stores local operator memory such as pins, notes, checks, and
connection history.

## Quick Start

```sh
make build
./bin/lazyss version
./bin/lazyss doctor
./bin/lazyss
```

On first launch, use the setup hints in the cockpit:

- `P` chooses an AWS profile from local AWS config.
- `L` runs `aws sso login` for the selected profile.
- `s` switches inventory source between all, SSH, and SSM.
- `r` refreshes inventory after profile/login changes.
- `f` opens structured filters with available tag suggestions.
- `m` cycles the access method for the selected machine.
- `?` opens the full key help overlay; `esc` closes any overlay.

LazySS stores profile and region labels only. It does not store AWS tokens, SSO
cache contents, passwords, private keys, or SSH config edits.

## Scope

V1 supports:

- read-only `~/.ssh/config` inventory
- AWS SSM + EC2 inventory
- direct `ssh` sessions
- `aws ssm start-session` sessions
- manual health checks
- local JSON state under the OS user config directory
- local hide/unhide for inventory cleanup
- guarded SSH config cleanup with dry-run, protected SCM identity hosts, and
  backup-before-write

V1 deliberately does not write ProxyCommand entries, run a monitoring daemon,
store secrets, delete private keys, or implement non-AWS cloud adapters.

## Usage

After installing `lazyss`:

```sh
lazyss
lazyss version
lazyss logo
lazyss logo --text "Ops Access"
lazyss doctor
lazyss --source all
lazyss --source ssh --ssh-config ~/.ssh/config
lazyss --source aws --aws-profile prod --aws-region ap-southeast-1
lazyss ssh cleanup --ssh-config ~/.ssh/config
lazyss ssh cleanup --ssh-config ~/.ssh/config --check
```

Inside the cockpit, use `P` to choose a local AWS profile and `L` to run
`aws sso login` for the selected profile. LazySS stores only the chosen profile
and region labels in local state; it never stores AWS credentials or SSO cache
contents.
Use `x` to hide or unhide a selected machine locally, and `u` to toggle hidden
machines in the current cockpit view.

The footer lists every cockpit key, and `?` opens the same list as a help
overlay. Use `m` to cycle a machine's access method and `c` to copy its connect
command. `g` runs a health check for the selected machine and `G` streams
bounded checks across all visible machines, updating rows as results arrive.
`h` toggles the detail panel, `e` opens an overlay to edit the note, tags, and
preferred method, and `v` shows the full session and health history. `C` opens
the guarded SSH config cleanup below.

SSH cleanup is dry-run by default. It hides SCM identity aliases such as GitHub
or Bitbucket from the machine cockpit, recommends port-forward aliases for
local hiding, and marks duplicate machine targets as delete candidates. To edit
the SSH config, pass explicit hosts with `--write`; LazySS writes a
`config.lazyss-backup-<timestamp>` file first and never removes private keys:

```sh
lazyss ssh cleanup --ssh-config ~/.ssh/config --host ts-workstation-name --write
```

## Architecture

LazySS uses pragmatic DDD boundaries. The domain model is provider-neutral, and
all cloud/terminal behavior sits behind ports so SSH, AWS SSM, and later cloud
variants can be added without rewriting the cockpit.

```txt
cmd/lazyss
  CLI flags, version/logo commands, TUI entrypoint
internal/domain
  Machine, access method, health, overlays, session history
internal/ports
  InventoryProvider, Connector, HealthChecker, StateStore interfaces
internal/app
  Inventory aggregation, connect, and health orchestration
internal/adapters/sshconfig
  Read-only SSH config inventory, ssh argv, TCP checks, cleanup planning
internal/adapters/awsssm
  AWS SSM/EC2 inventory, aws ssm start-session argv, SSM health
internal/adapters/statejson
  Local 0600 JSON operator memory
internal/tui
  Bubble Tea cockpit over app use cases only
internal/doctor
  Local tool, AWS credential, region, and plugin readiness checks
internal/brand
  Product name, ASCII logo, and version report rendering
```

The main contexts are:

- **Machine Inventory:** SSH config and AWS SSM/EC2 discovery.
- **Access:** argv-only session command construction and terminal handoff.
- **Health:** method-specific readiness observations.
- **Operator Memory:** local pins, hide state, notes, preferred method, checks,
  and connection history.
- **Cockpit:** TUI rendering and keyboard flow over app-layer use cases.
- **Preflight:** `lazyss doctor` checks for local dependencies and AWS setup.

## Versioning

Releases use SemVer tags such as `v0.1.0`. Local builds use `git describe`, so
developer binaries show values such as `v0.1.0-12-gabcdef` or
`v0.1.0-12-gabcdef-dirty`.

```sh
lazyss --version
lazyss version
```

`--version` stays script-friendly and prints one line. `version` prints the
product name, binary name, and resolved build version for human inspection.
The Makefile injects the value with:

```txt
-X main.version=$(git describe --tags --always --dirty)
```

GoReleaser injects the tagged release version during release builds.

## Branding

`lazyss logo` prints the default ASCII mark used by the README and setup panel.
`lazyss logo --text "<label>"` creates a compact ASCII banner for terminal
notes, runbooks, demos, or internal docs without adding a graphics dependency.

```sh
lazyss logo
lazyss logo --text "Ops Access"
```

## Install

### From a Local Checkout

```sh
make build
./bin/lazyss --version
```

### From GitHub Releases

After `v0.1.0`, download the archive for your OS and architecture from
[GitHub Releases](https://github.com/hmrdkn-labs/lazyss/releases), verify it
against `checksums.txt`, and put `lazyss` on your `PATH`.

### Homebrew

After the public Homebrew tap is published:

```sh
brew install --formula hmrdkn-labs/tap/lazyss
```

The formula is the primary macOS/Linux install path. Unsigned cask binaries can
be blocked by macOS quarantine, so LazySS publishes a formula until the project
has Developer ID signing and notarization.

### Go Install

Public Go module install:

```sh
go install github.com/hmrdkn-labs/lazyss/cmd/lazyss@latest
```

## Verification

```sh
gofmt -l .
go vet ./...
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -1
go build ./cmd/lazyss
make smoke-local
make smoke-tui
```

## Release Status

`v0.1.0` must not be tagged until local gates, fast hosted CI, the
release-candidate workflow, Homebrew readiness, and real SSH/AWS SSM smoke
tests pass. After publishing, verify a clean Homebrew formula install from the
public tap.

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
```

Before asking for owner approval on branch protection, tap creation, repository
secrets, local tap setup, or tagging, generate the ignored local handoff:

```sh
make release-approval-plan
```

Do not put secrets, private keys, token values, SSO cache data, environment
dumps, release asset URLs with credentials, or full terminal logs in evidence
files.
