# Brief: tui-wiring (lazyss v0.3)

Worktree: `~/workspace/.worktrees/lazyss/tui-wiring`, branch `feat/tui-wiring`, based on `main` after
`feat/gcp-adapter` and `feat/azure-adapter` have merged. Read first: `CONTRIBUTING.md`, `internal/ports/ports.go`,
`internal/adapters/gcloud/`, `internal/adapters/azcli/` (exported API and doc comments), `internal/app/app.go`,
`cmd/lazyss/main.go`, `internal/tui/{model,keymap,cockpit,picker,filter,listview}.go`, `internal/doctor/doctor.go`,
`scripts/smoke-local.sh`, `scripts/smoke-tui.sh`.

## Owner decisions (do not reopen)

- Google Cloud and Azure are new sources, backed by the `gcloud`/`az` CLI adapters already on `main`.
  No Go cloud SDKs. lazyss never reads or stores cloud tokens.
- A missing CLI or login shows a clear provider status in the UI and in `lazyss doctor`. It never crashes
  lazyss and never blocks the other providers.
- `--source` gains `gcp` and `azure`; the source cycle (`s`) and method cycle (`m`) include them.

## Files you own

`cmd/lazyss/`, `internal/app/`, `internal/tui/`, `internal/doctor/`, `internal/ports/ports.go`,
`internal/domain/domain.go` (only `OperatorPreferences`), `internal/adapters/statejson/` (only if preferences
need it), `scripts/smoke-*.sh`. Do not change the adapter packages except to fix a bug you report;
do not touch README/ADR/CHANGELOG/site (another brief).

## What to build

1. **Non-blocking inventory.** `app.InventoryService.List` queries providers concurrently and keeps the
   output deterministic (statuses in provider order, machines sorted as today). A slow or failing provider
   only degrades its own status. Race-tested with a slow fake provider.
2. **CLI flags** in `cmd/lazyss/main.go`: `--source all|ssh|aws|gcp|azure`, `--gcp-project`,
   `--azure-subscription`. Defaults: GCP uses `ActiveProject` when no flag/preference; Azure uses the CLI's
   default subscription (empty string). Wire `gcloud.NewInventory`, `azcli.NewInventory`, both connectors and
   both checkers (3 s dial timeout like SSH). If a CLI is missing, the provider is still registered and
   reports its degraded status (no `failingProvider` crash path for these).
3. **Preferences.** Add `GCPProject` and `AzureSubscription` to `domain.OperatorPreferences` (safe identifiers
   only) and persist them the same way the AWS profile is persisted.
4. **TUI.**
   - Source cycle `s`: all → ssh → aws → gcp → azure → all. Update the `s` help text.
   - Method cycle `m` cycles only through the selected machine's `Methods` (verify it already does; add
     labels for the four new methods wherever methods are rendered). `filter.go` method aliases:
     `gcloud`/`gcp` → `gcloud-ssh`, `iap` → `gcloud-iap-ssh`, `az`/`azure` → `az-ssh`, `bastion` → `az-bastion-ssh`.
   - Project/subscription picker on the existing `P` key: it picks for the provider of the current source
     (aws → AWS profiles as today; gcp → `gcloud.ListProjects`; azure → `azcli.ListSubscriptions`, show name
     and id). When the source is `all`, use the selected machine's provider; with no selection, AWS as today.
     Selecting rebuilds that provider's inventory like `SetAWSProfile` does. Add ports interfaces so the TUI
     does not import adapters directly.
   - Login on the existing `L` key, chosen the same way: AWS SSO login as today; gcp runs
     `gcloud auth login`; azure runs `az login` (interactive via `tea.Exec`, from each adapter's
     `LoginCommand()`). Refresh inventory after a successful login.
   - Provider warnings in the cockpit header for gcp/azure auth/missing-CLI statuses, in the style of the AWS
     one ("source gcp degraded: not logged in; L login"). Header shows the active GCP project and Azure
     subscription when those sources are in view.
   - Help overlay lists everything above.
5. **Doctor.** Add gcloud and azure sections using `gcloud.CLIStatus` and `azcli.CLIStatus`. A missing cloud CLI
   is reported clearly ("not installed; optional, needed for --source gcp") and must not change doctor's
   exit status unless the user explicitly asked for that source. Never print tokens or account object ids.
6. **Smoke.** Keep `make smoke-local` passing; extend it to assert `--source gcp` and `--source azure` parse,
   and that `doctor` output contains the new sections, without any real cloud login (PATH without gcloud/az
   is fine: the status must say not installed).

## Tests

Model tests for source cycle, picker per provider, login dispatch per provider, provider warnings; main tests
for flag parsing and invalid source; doctor tests with fake LookPath/status; app concurrency test. Keep total
coverage ≥ `coverage.baseline`; raise the baseline in the same commit if coverage went up.

## Checks to pass before reporting

`make check`, `make smoke-local`, `make lint` if golangci-lint is installed. Conventional commits.
Do not push, merge or tag.

## Report (fill in, then reply in ≤ 20 lines)

- Commits:
- `make check` / `make smoke-local` result (last lines):
- Total coverage vs baseline:
- Key bindings and help text changed:
- Anything the reviewer should look at:
