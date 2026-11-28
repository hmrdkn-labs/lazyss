# Brief: azure-adapter (lazyss v0.3)

Worktree: `~/workspace/.worktrees/lazyss/azure-adapter`, branch `feat/azure-adapter`, based on `main`.
Read first: `CONTRIBUTING.md`, `internal/ports/ports.go`, `internal/adapters/awsssm/awsssm.go`,
`internal/adapters/sshconfig/sshconfig.go` (the `Checker`/`Dialer` pattern), `internal/app/app.go`.
`az` is not installed on this Mac; everything is tested with fixtures and a fake runner.

## Owner decisions (do not reopen)

- Everything goes through the `az` CLI via `exec` with an argv slice (never a shell string), `-o json`.
- No Go cloud SDKs, no new modules in `go.mod`. lazyss never reads or stores tokens; it reuses the user's
  `az` login.
- A missing CLI or missing login yields a degraded `domain.ProviderStatus` with a clear message. It never
  panics, never blocks other providers (bound every CLI call with a context timeout, 30 s for inventory,
  10 s for metadata calls).

## Files you own

- `internal/adapters/azcli/` (new package `azcli`): code, tests, `testdata/*.json` fixtures.
- `internal/domain/domain.go`: ONLY the two const blocks below, replaced verbatim. The gcp worker adds the
  identical text on its branch so the merge is clean. Do not change anything else in domain.

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
- `NewInventory(runner Runner, subscription string) Inventory`; `ProviderName() == "azure"`.
  `ListMachines` runs three calls (each with `--subscription <s>` when set):
  1. `az vm list -d -o json`: name, resourceGroup, location, id, powerState (`"VM running"` etc.),
     publicIps, privateIps (comma-separated strings in `-d` output), tags → `ProviderTags`,
     storageProfile.osDisk.osType → Platform, networkProfile.networkInterfaces[].id.
  2. `az network nic list -o json`: map NIC id → subnet id(s) → VNet id (`.../virtualNetworks/<vnet>`).
  3. `az network bastion list -o json`: Bastion name, resourceGroup, ipConfigurations[].subnet.id → VNet id.
  A VM gets `AccessAzBastionSSH` only when a Bastion sits in the same VNet as one of its NICs (peered VNets
  are a documented limitation in a doc comment). If the nic or bastion call fails, still return the VMs with
  only `AccessAzSSH` and put the reason in the status message (status stays healthy if the VM list worked).
  Machine fields: `Provider: domain.ProviderAzure`, `NativeID`: VM resource id,
  `ID: domain.MachineID("azure:<subscription>/<resourceGroup>/<name>")` (subscription parsed from the resource
  id when not given), `Scope{Account: subscription, Region: location, Profile: resourceGroup}`,
  `Address`: first public IP else first private IP, `State`: power state.
  Methods: `[AccessAzSSH]` plus `AccessAzBastionSSH` when a Bastion matches; put Bastion first when the VM has
  no public IP. Carry the matched Bastion to the connector in the existing `domain.Machine.Metadata` map
  (not `ProviderTags`, which is user data): keys `azure.bastion.name` and `azure.bastion.resourceGroup`.
  Export those keys as constants in `azcli`.
- Initial `Health` from power state: `VM running` → up "azure VM running"; others → down with the state.
  The label never claims login works.
- Error classification for the provider status message (tested): executable not found (`exec.ErrNotFound`)
  → "az CLI not found on PATH"; stderr containing `az login` / `Please run 'az login'` /
  `AADSTS` → "az not logged in: run az login (press the login key)"; others → truncated stderr first line.
  Use `*exec.ExitError.Stderr`.
- `CLIStatus(ctx, runner) domain.ProviderStatus` for `lazyss doctor`: CLI present? `az account show -o json`
  succeeds? Message names the subscription name and id only (never tenant tokens, never user object ids).
  Also report whether the `ssh` extension is installed (`az extension list -o json` contains `ssh`), since
  `az ssh vm` needs it.
- `ListSubscriptions(ctx, runner) ([]Subscription, error)` from `az account list -o json` (id, name,
  isDefault, state).
- `LoginCommand() ports.CommandSpec` → `az login`.
- `Connector`: `Supports` for the two Azure methods on Azure machines. `BuildCommand` argv (golden-tested):
  - Entra ID: `az ssh vm --resource-group <rg> --name <vm> --subscription <sub>`
  - Bastion: `az network bastion ssh --name <bastion> --resource-group <bastionRG> --target-resource-id <vmId> --auth-type AAD --subscription <sub>`
  Reject any value starting with `-` (argv injection) with an error; test it.
- `Checker` (inject a `Dialer` like sshconfig, timeout ≤ 3 s):
  - `az-ssh`: TCP connect to `<public IP>:22` bounded at 3 s ("tcp/22 open" proves only that the port
    accepted a TCP connection, not that Entra login or RBAC works). No public IP → report power state as
    unknown/down "no public IP; power state <x>".
  - `az-bastion-ssh`: power state plus Bastion presence from inventory only ("VM running, bastion <name>").
    It does not prove Bastion SKU supports native client, nor RBAC. Do not shell out to `az` in a check.
  - Put a doc comment on each check saying exactly what it proves and what it does not.

## Tests (no network, no real az)

- Fixture JSON under `testdata/` for vm list -d (≥3 VMs: public IP, private-only in a Bastion VNet,
  deallocated), nic list, bastion list, account list, account show, extension list.
- Fake runner recording argv; assert exact argv for every CLI call and every connect command (golden).
- Error classification table test; nic/bastion failure degrades to az-ssh only.
- Checker tests with a fake dialer.
- Package coverage ≥ 80 %. Total coverage must not drop below `coverage.baseline`.

## Checks to pass before reporting

`make check` and `make lint` if golangci-lint is installed. Conventional commits (`feat(azcli): ...`).
Do not push, do not merge, do not tag.

## Report (fill in, then reply in ≤ 20 lines)

- Commits:
- `make check` result (last lines):
- Package coverage / total coverage:
- Anything the reviewer should look at:
