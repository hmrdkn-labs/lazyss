# Brief: gcp-adapter (lazyss v0.3)

Worktree: `~/workspace/.worktrees/lazyss/gcp-adapter`, branch `feat/gcp-adapter`, based on `main`.
Read first: `CONTRIBUTING.md`, `internal/ports/ports.go`, `internal/adapters/awsssm/awsssm.go`,
`internal/adapters/sshconfig/sshconfig.go` (the `Checker`/`Dialer` pattern), `internal/app/app.go`.

## Owner decisions (do not reopen)

- Everything goes through the `gcloud` CLI via `exec` with an argv slice (never a shell string), `--format=json`.
- No Go cloud SDKs, no new modules in `go.mod`. lazyss never reads or stores tokens; it reuses the user's
  `gcloud` login.
- A missing CLI or missing login yields a degraded `domain.ProviderStatus` with a clear message. It never
  panics, never blocks other providers (bound every CLI call with a context timeout, 20 s for inventory,
  10 s for metadata calls).

## Files you own

- `internal/adapters/gcloud/` (new package `gcloud`): code, tests, `testdata/*.json` fixtures.
- `internal/domain/domain.go`: ONLY the two const blocks below, replaced verbatim. The azure worker adds
  the identical text on its branch so the merge is clean. Do not change anything else in domain.

```go
// Provider constants identify supported inventory providers.
const (
	ProviderSSH   ProviderKind = "ssh"
	ProviderAWS   ProviderKind = "aws"
	ProviderGCP   ProviderKind = "gcp"
	ProviderAzure ProviderKind = "azure"
)
```

```go
// Access method constants identify supported connection launch modes.
const (
	AccessSSH          AccessMethod = "ssh"
	AccessAWSSSMShell  AccessMethod = "aws-ssm-shell"
	AccessSSHOverSSM   AccessMethod = "ssh-over-ssm"
	AccessGCloudSSH    AccessMethod = "gcloud-ssh"
	AccessGCloudIAPSSH AccessMethod = "gcloud-iap-ssh"
	AccessAzSSH        AccessMethod = "az-ssh"
	AccessAzBastionSSH AccessMethod = "az-bastion-ssh"
)
```

Do not touch `cmd/`, `internal/tui`, `internal/app`, `internal/doctor`, `internal/ports`, docs. Wiring is a later brief.

## What to build (exported API the tui-wiring brief will call)

- `Runner` interface = `RunOutput(ctx, exe string, args []string) ([]byte, error)` and
  `RunInteractive(ctx, exe string, args []string) error` (matches `ports.CommandRunner`), plus an unexported
  `osRunner`. Constructors accept a nil runner and default to `osRunner`.
- `NewInventory(runner Runner, project string) Inventory`; `ProviderName() == "gcp"`.
  `ListMachines` runs `gcloud compute instances list --format=json` (add `--project <p>` when set).
  Map each instance: name, zone (last path segment of `zone` URL), status, internal IP
  (`networkInterfaces[0].networkIP`), external IP (`accessConfigs[].natIP`), labels → `ProviderTags`,
  `Scope{Account: project, Region: zone}`, `Provider: domain.ProviderGCP`, `NativeID: name`,
  `ID: domain.MachineID("gcp:<project>/<zone>/<name>")`, `Address`: external IP else internal IP.
  When project is empty, take the project from the instance `selfLink` (`projects/<p>/zones/...`).
  Methods: `[AccessGCloudSSH, AccessGCloudIAPSSH]` when an external IP exists, else `[AccessGCloudIAPSSH, AccessGCloudSSH]`
  (IAP first because direct needs a reachable IP; `gcloud compute ssh --internal-ip` is out of scope).
  Initial `Health`: provider status (RUNNING → up "gcp RUNNING", others → down with the status, label never
  claims login works).
- Error classification for the provider status message (tested): executable not found
  (`exec.ErrNotFound`) → "gcloud CLI not found on PATH"; stderr containing auth errors
  (`gcloud auth login`, `You do not currently have an active account`, `Reauthentication`) →
  "gcloud not logged in: run gcloud auth login (press the login key)"; no project configured → clear message;
  others → truncated stderr first line. Use `*exec.ExitError.Stderr`.
- `CLIStatus(ctx, runner) domain.ProviderStatus` for `lazyss doctor`: CLI present?
  `gcloud auth list --format=json` has an ACTIVE account? Message names the active account only, nothing else.
- `ListProjects(ctx, runner) ([]Project, error)` from `gcloud projects list --format=json` (projectId, name),
  and `ActiveProject(ctx, runner) (string, error)` from `gcloud config get-value project --format=json`
  (empty string when unset, not an error).
- `LoginCommand() ports.CommandSpec` → `gcloud auth login` (interactive, run by the TUI later).
- `Connector`: `Supports` for the two GCP methods on GCP machines. `BuildCommand` argv (golden-tested):
  - direct: `gcloud compute ssh <name> --zone <zone> --project <project>`
  - IAP: `gcloud compute ssh <name> --zone <zone> --project <project> --tunnel-through-iap`
  Reject names/zones/projects starting with `-` (argv injection) with an error; test it.
- `Checker` (inject a `Dialer` like sshconfig, timeout ≤ 3 s):
  - direct: TCP connect to `<external IP>:22` bounded at 3 s. Up label "tcp/22 open" means only that the
    port accepted a TCP connection; no external IP → unknown "no external IP".
  - IAP: reports provider status only (RUNNING → up "gcp RUNNING (IAP not probed)"). It does not prove the
    IAP firewall rule or IAM permission. Do not shell out to gcloud in a health check.
  - Put a doc comment on each check saying exactly what it proves and what it does not.

## Tests (no network, no real gcloud)

- Fixture JSON under `testdata/` (≥3 instances: external IP, internal-only, TERMINATED; labels).
- Fake runner recording argv; assert exact argv for every CLI call and every connect command (golden).
- Error classification table test (not found, not logged in, generic).
- Checker tests with a fake dialer (open, refused, no IP, IAP running/stopped).
- Package coverage ≥ 80 %. Total coverage must not drop below `coverage.baseline`.

## Checks to pass before reporting

`make check` (fmt, tidy, vet, race tests + coverage baseline, script tests, build) and `make lint` if
golangci-lint is installed. Use conventional commits (`feat(gcloud): ...`, `test(gcloud): ...`).
Do not push, do not merge, do not tag.

## Report (fill in, then reply in ≤ 20 lines)

- Commits:
- `make check` result (last lines):
- Package coverage / total coverage:
- Anything the reviewer should look at:
