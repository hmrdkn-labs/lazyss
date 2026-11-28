# Brief: release-0.3 (lazyss v0.3.0 docs and readiness)

Worktree: `~/workspace/.worktrees/lazyss/release-0.3`, branch `docs/release-0.3`, based on `main` after
`feat/tui-wiring` has merged. Doc-only and read-only release prep: no Go code changes.
Read first: `CONTRIBUTING.md`, `README.md`, `CHANGELOG.md`, `docs/adr/*.md`, `docs/runbooks/release.md`,
`docs/runbooks/homebrew.md`, `docs/runbooks/smoke.md`, the `Makefile`, `scripts/release-readiness.sh`,
`scripts/homebrew-readiness.sh`, and the code in `internal/adapters/gcloud`, `internal/adapters/azcli`,
`cmd/lazyss/main.go`, `internal/tui/keymap.go` (every doc sentence must match the code).

## Owner decisions (do not reopen)

- v0.3 adds Google Cloud and Azure through the `gcloud`/`az` CLIs (argv exec, JSON output), not Go SDKs;
  lazyss reuses the user's CLI login and never reads or stores tokens.

## Files you own

`README.md`, `CHANGELOG.md`, `docs/adr/0004-cli-backed-cloud-providers.md` (new), `docs/adr/0001-architecture.md`
(only to point at 0004 where it limits non-AWS adapters, if it does), `docs/runbooks/*.md` (only where they name
supported providers or smoke paths), `docs/briefs/release-0.3.md` (Report).

## What to do

1. README: Scope and Usage cover `--source gcp|azure`, `--gcp-project`, `--azure-subscription`, the four new
   methods and how each connects (exact commands the connectors build), `P`/`L` per provider, doctor sections,
   and what each health check proves and does not prove (copy from the adapter doc comments).
2. ADR 0004 (Status: Accepted, date 2026-11-28): context, decision (CLI-backed providers, no SDKs, no token
   handling), consequences (CLI install/login is the user's, CLI output-format drift risk, test strategy with
   fake runners and fixtures, Bastion detection limited to same-VNet). Find any sentence in existing docs that
   says there are no non-AWS adapters and point it at 0004.
3. CHANGELOG: a `v0.3.0` section in the file's existing style, dated when tagged (leave `Unreleased` heading if
   that is the file's convention).
4. Smoke runbook / live-smoke evidence: if the runbook or `scripts/live_smoke_evidence.py` lists required live
   paths, document GCP/Azure live smoke as optional evidence for v0.3.0 (owner decides whether it is required;
   say so in the Report). Do not change Python scripts.
5. Read-only readiness: run `make check`, `make workflow-policy`, `make release-approval-plan`,
   `./scripts/homebrew-readiness.sh`, and `LAZYSS_RELEASE_VERSION=v0.3.0 make release-preflight`. Record each
   exit code and the blocker lines (no secrets) in the Report. Check how the Homebrew formula in
   `~/workspace/labs/homebrew-tap` gets bumped (GoReleaser or manual) and write the exact steps in the Report;
   do not edit or push the tap.

Never create tags, releases, secrets, branch protection, or push anything.

## Checks to pass before reporting

`make check` still green (docs only). Conventional commits (`docs: ...`).

## Report (fill in, then reply in ≤ 20 lines)

- Commits:
- Readiness commands and exit codes, with blocker lines:
- Homebrew tap bump steps:
- Owner actions needed before tagging v0.3.0:
