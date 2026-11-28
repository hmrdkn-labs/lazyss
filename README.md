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

`lazyss` is a terminal machine cockpit for direct SSH, AWS Systems Manager
Session Manager, Google Cloud Compute Engine, and Azure VMs. It discovers
machines from read-only SSH config, AWS SSM/EC2, `gcloud`, and `az` inventory,
shows method-specific reachability, launches secure sessions, and stores local
operator memory such as pins, notes, checks, and connection history.

## Quick Start

```sh
make build
./bin/lazyss version
./bin/lazyss doctor
./bin/lazyss
```

On first launch, use the setup hints in the cockpit:

- `P` chooses the AWS profile, GCP project, or Azure subscription for the provider in view.
- `L` logs in to that provider with `aws sso login`, `gcloud auth login`, or `az login`.
- `s` switches inventory source between all, SSH, SSM, GCP, and Azure.
- `r` refreshes inventory after profile/login changes.
- `f` opens structured filters with available tag suggestions.
- `m` cycles the access method for the selected machine.
- `?` opens the full key help overlay; `esc` closes any overlay.

Provider preferences store only the AWS profile and region labels, the GCP
project ID, and the Azure subscription ID. LazySS does not store AWS tokens, SSO
cache contents, gcloud or az credentials, passwords, private keys, or SSH config
edits.

## Scope

V1 supports:

- read-only `~/.ssh/config` inventory
- AWS SSM + EC2 inventory
- Google Cloud Compute Engine inventory through `gcloud`
- Azure VM inventory through `az`, with same-VNet Bastion matching
- direct `ssh` sessions
- `aws ssm start-session` sessions
- `gcloud compute ssh` sessions, direct or through IAP
- `az ssh vm` and `az network bastion ssh` sessions
- manual health checks
- local JSON state under the OS user config directory
- local hide/unhide for inventory cleanup
- guarded SSH config cleanup with dry-run, protected SCM identity hosts, and
  backup-before-write

V1 deliberately does not write ProxyCommand entries, run a monitoring daemon,
store secrets, or delete private keys. The Google Cloud and Azure adapters use
no cloud SDKs: they run the `gcloud` and `az` CLIs and reuse the operator's
login; see [ADR 0004](docs/adr/0004-cli-backed-cloud-providers.md). Other cloud
providers are not implemented.

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
lazyss --source gcp --gcp-project <project-id>
lazyss --source azure --azure-subscription <subscription-id-or-name>
lazyss ssh cleanup --ssh-config ~/.ssh/config
lazyss ssh cleanup --ssh-config ~/.ssh/config --check
```

Inside the cockpit, `P` and `L` act on the provider in view:

- `--source gcp`, or `all` with a GCP machine selected: `P` lists projects from
  `gcloud projects list`; `L` runs `gcloud auth login`.
- `--source azure`, or `all` with an Azure machine selected: `P` lists
  subscriptions from `az account list`; `L` runs `az login`.
- Otherwise: `P` lists local AWS profiles and `L` runs `aws sso login` for the
  selected profile.

A choice made with `P` is saved and used on the next start unless a flag
overrides it. Provider preferences store only the AWS profile and region, the GCP
project ID, and the Azure subscription ID in local state; LazySS never stores AWS
credentials, SSO cache contents, or gcloud or az tokens.

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

### Google Cloud and Azure

`--source gcp` lists Compute Engine instances with
`gcloud compute instances list --format=json`. `--source azure` lists VMs with
`az vm list -d -o json`, then matches Bastions using
`az network nic list -o json` and `az network bastion list -o json`. Both add
`--project <id>` or `--subscription <id>` when a flag or saved choice sets one;
otherwise the CLI default applies. LazySS runs these commands with your existing
login and never reads its tokens.

| Method | Offered when | `Enter` runs |
| --- | --- | --- |
| `gcloud-ssh` | always; first when the instance has an external IP | `gcloud compute ssh <instance> --zone <zone> --project <project>` |
| `gcloud-iap-ssh` | always; first when there is no external IP | the same command with `--tunnel-through-iap` |
| `az-ssh` | always; second only for a VM with no public IP that matches a Bastion | `az ssh vm --resource-group <rg> --name <vm> --subscription <sub>` |
| `az-bastion-ssh` | a Bastion sits in the VM's VNet | `az network bastion ssh --name <bastion> --resource-group <bastion-rg> --target-resource-id <vm-id> --auth-type AAD --subscription <sub>` |

`az ssh vm` needs the `ssh` extension (`az extension add --name ssh`). Bastion
matching covers only a Bastion in the same VNet as one of the VM's NICs; Bastions
reached through VNet peering are not detected. If the NIC or Bastion list call
fails, Azure VMs keep their `az-ssh` method, Bastion matching is off, and the
reason is recorded in the provider status message.

Health checks (`g` for the selected machine, `G` for all visible machines) prove
only what each row says:

| Method | Probe | Proves | Does not prove |
| --- | --- | --- | --- |
| `gcloud-ssh` | TCP connect to the external IP on port 22, 3 s max | `tcp/22 open`: the port accepted a TCP connection from this host | SSH authentication, OS Login, or key acceptance. No external IP is reported as unknown. |
| `gcloud-iap-ssh` | none; shows the inventory status, `gcp RUNNING (IAP not probed)` | the instance state at the last listing | the IAP firewall rule, IAM permission, or login. No gcloud call is made. |
| `az-ssh` | TCP connect to the public IP on port 22, 3 s max; a VM without a public IP is not dialed | `tcp/22 open`: the port accepted a TCP connection from this host | Entra ID login, RBAC, or the AADSSHLogin VM extension. `tcp/22 refused` only for ECONNREFUSED; timeouts and unreachable networks are `tcp/22 probe failed`. |
| `az-bastion-ssh` | none; uses inventory power state and Bastion match, no `az` call | the VM power state and a Bastion matched in its VNet | Bastion SKU native-client tunnelling support, RBAC, or Entra login |

`lazyss doctor` always adds `gcloud` and `azure` checks after the AWS checks.
AWS checks remain mandatory even for cloud-only doctor invocations. A missing or
logged-out CLI prints `[warn]` and does not change the exit code. With
`--source gcp` or `--source azure`, a *failed* matching check becomes fatal and
exits 1; the source flag alone does not.
The gcloud check reports `logged in as <account>` only for an ACTIVE entry in
`gcloud auth list --format=json`. The azure check reports the active
subscription name, with parenthesised GUIDs removed, and whether the `ssh`
extension is installed; when it is missing, the detail names
`az extension add --name ssh`.
Neither check verifies a token refresh, inventory authorization, SSH
authorization, or session launch, so an active or cached account is not proof
that credentials still work. `--gcp-project` and `--azure-subscription` do not
scope doctor.

## Architecture

LazySS uses pragmatic DDD boundaries. The domain model is provider-neutral, and
all cloud/terminal behavior sits behind ports so SSH, AWS SSM, Google Cloud,
Azure, and later cloud variants can be added without rewriting the cockpit.

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
internal/adapters/gcloud
  gcloud Compute Engine inventory, gcloud compute ssh argv, TCP and status health
internal/adapters/azcli
  az VM and Bastion inventory, az ssh and az network bastion ssh argv, TCP health
internal/adapters/statejson
  Local 0600 JSON operator memory
internal/tui
  Bubble Tea cockpit over app use cases only
internal/doctor
  Local tool, AWS credential, region, plugin, and gcloud/az login readiness checks
internal/brand
  Product name, ASCII logo, and version report rendering
```

The main contexts are:

- **Machine Inventory:** SSH config, AWS SSM/EC2, gcloud, and az discovery.
- **Access:** argv-only session command construction and terminal handoff.
- **Health:** method-specific readiness observations.
- **Operator Memory:** local pins, hide state, notes, preferred method, checks,
  and connection history.
- **Cockpit:** TUI rendering and keyboard flow over app-layer use cases.
- **Preflight:** `lazyss doctor` checks for local dependencies, AWS setup, and gcloud/az login.

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

`v0.3.0` must not be tagged until local gates, fast hosted CI, the
release-candidate workflow, Homebrew readiness, and real SSH/AWS SSM smoke
tests pass. Google Cloud and Azure smoke is optional evidence for v0.3.0; see
the smoke runbook. After publishing, verify a clean Homebrew formula install
from the public tap.

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
