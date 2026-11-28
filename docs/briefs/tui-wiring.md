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

## Process safety (applies to every brief)

Never kill processes by name (`pkill -f`, `killall`, e.g. wrangler/workerd): other agents run production
processes on this Mac. Start any dev server or background process with `cmd & echo $! > <pidfile>` and kill
only that PID and its children. Do not use port 8787 (taken by another agent); pick another free port.

## Report (fill in, then reply in ≤ 20 lines)

- Commits: `d9e5bf5` style(azcli) tagged switch (lint QF1002); `feat(tui)` commit (see `git log`) with everything else.
- `make check` / `make smoke-local` result (last lines): check: `ok coverage 78.7% meets baseline`, 43 script tests OK; smoke-local: `local smoke passed`. golangci-lint v2.12.2: 0 issues.
- Total coverage vs baseline: 78.7% vs 57.7%; baseline raised to 78.5.
- Key bindings and help text changed: `s` cycles all/ssh/ssm/gcp/azure; `P` (footer "scope") picks AWS profile / GCP project / Azure subscription; `L` runs AWS SSO / `gcloud auth login` / `az login` via tea.ExecProcess. Target = source in view, or for `all` the selected machine's provider, else AWS. `m` already cycled only the machine's Methods (test added). Help panel widened to 64.
- Anything the reviewer should look at: `InventoryService.List` now concurrent (per-index results, provider order kept). Providers gcp/azure always registered; missing CLI shows as degraded status. Doctor: new `Check.Optional` ("warn" mark, no exit change unless `--source gcp|azure`); az GUIDs in parentheses are redacted. Provider badge for azure is "az" (3-col list column); method labels `gcloud-iap`/`az-bastion` shortened in list/detail/editor. Startup calls `gcloud.ActiveProject` when source is all/gcp and no flag/pref (one extra gcloud spawn). Preferences save now writes all four fields so P on one provider no longer clobbers others.

### Review fixes (review-tui-wiring.md)

- Blocker: `InventoryService.Stream` emits a cumulative snapshot per finished provider (statuses in provider order, machines sorted); `List` is its last snapshot. TUI `machinesMsg` carries the stream and re-arms until it closes. Tests: `TestInventoryStreamEmitsFastProvidersBeforeBlockedOne` (blocked fake provider), `TestModelRendersFastProviderWhileSlowOneIsBlocked`.
- Should-fix: default GCP project resolved in the background (`Runtime.ResolveGCPProject`, `gcpProjectMsg`; never overwrites a picked project); `buildProviders` registers ssh/aws/gcp/azure for every `--source` (AWS config loads lazily on first listing, so startup is not delayed); test registers all providers for each restricted source. `smoke-tui.sh` help assertion now matches "selected machine".
- azcli nit: ignored per orchestrator exception.
- Checks: `make check` ok (78.6% vs baseline 78.5, 43 script tests), golangci-lint v2.12.2 0 issues, `make smoke-local` passed, `make smoke-tui` all assertions passed.
