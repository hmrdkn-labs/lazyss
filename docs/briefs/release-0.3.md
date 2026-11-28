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

## Process safety (applies to every brief)

Never kill processes by name (`pkill -f`, `killall`, e.g. wrangler/workerd): other agents run production
processes on this Mac. Start any dev server or background process with `cmd & echo $! > <pidfile>` and kill
only that PID and its children. Do not use port 8787 (taken by another agent); pick another free port.

## Report (fill in, then reply in ≤ 20 lines)

- Commits:
  - `4fd24f5d1f` `docs: describe v0.3 Google Cloud and Azure support` (README, CHANGELOG, ADR 0004, smoke and release runbooks). ADR 0001 left unchanged: it lists V1 adapters but does not exclude non-AWS ones, so no pointer added.
  - Second commit: this Report (`docs: record release-0.3 readiness`).
  - Check after last edit: `make check` rc 0 (go test -race, coverage 78.7% vs baseline 78.5%, 43 script tests OK, build).
- Readiness commands and exit codes, with blocker lines:
  - `make workflow-policy`: rc 0.
  - `make release-approval-plan`: rc 0 (writes gitignored `release-approval.md`).
  - `./scripts/homebrew-readiness.sh`: rc 0. Passed: tap `hmrdkn-labs/homebrew-tap` public, `HOMEBREW_TAP_GITHUB_TOKEN` name present, `hmrdkn-labs/tap` tapped locally. Warn: goreleaser not installed locally.
  - `LAZYSS_RELEASE_VERSION=v0.3.0 make release-preflight`: rc 2 (script exit 2; make reports Error 2). Blockers at HEAD `4fd24f5d1f`:
    - `[blocker] not on main: docs/release-0.3`
    - `[blocker] local HEAD does not match origin/main`
    - `[blocker] 8 open pull request(s) remain`
    - `[blocker] CI latest run is for cb61cbc25f95, not current main 4fd24f5d1f77`
    - `[blocker] Release Candidate latest run is for 987e22322bc5, not current main 4fd24f5d1f77`
    - `[blocker] live smoke evidence file is not provided; set LAZYSS_LIVE_SMOKE_EVIDENCE after real SSH/AWS SSM smokes`
    - Passed: clean tree, public repo, branch protection, no local or GitHub `v0.3.0` yet, Homebrew readiness, ssh/aws/session-manager-plugin present.
- Homebrew tap bump steps:
  - Automated in `.github/workflows/release.yml` on tag push `v*.*.*`: tag-mode readiness audit; GoReleaser release publishes archives and `checksums.txt` to the GitHub Release (`.goreleaser.yaml` has no `brews:`, so GoReleaser does not write the tap); then "Update Homebrew formula" runs `scripts/homebrew_formula.py generate --dist dist --version <tag> --output homebrew-tap/Formula/lazyss.rb`, commits as `github-actions[bot]` with `brew: update lazyss <tag>`, and pushes to tap `main` using `HOMEBREW_TAP_GITHUB_TOKEN`.
  - Evidence it is automated, not manual: tap commits `cbb861d brew: update lazyss v0.2.0` and `569ac41 brew: update lazyss v0.1.1` are authored by github-actions[bot]. Local `~/workspace/labs/homebrew-tap` `Formula/lazyss.rb` is still v0.2.0, as expected before a v0.3.0 tag.
  - Manual fallback, only if that workflow step fails (not run). Use the published release assets, not `make release-snapshot` output, whose archive names and checksums [INFERENCE: differ from the published ones]:
    ```sh
    TAG=v0.3.0
    DIST=$(mktemp -d)
    gh release download "$TAG" --repo hmrdkn-labs/lazyss --dir "$DIST" \
      --pattern checksums.txt --pattern 'lazyss_*_darwin_*.tar.gz' --pattern 'lazyss_*_linux_*.tar.gz'
    cd ~/workspace/labs/homebrew-tap && git pull --ff-only origin main
    python3 ~/workspace/.worktrees/lazyss/release-0.3/scripts/homebrew_formula.py generate \
      --dist "$DIST" --version "$TAG" --output Formula/lazyss.rb
    git add Formula/lazyss.rb && git commit -m "brew: update lazyss $TAG" && git push origin main
    brew update && brew install --formula hmrdkn-labs/tap/lazyss && lazyss --version
    ```
- Owner actions needed before tagging v0.3.0:
  - Merge `docs/release-0.3` to `main`; then HEAD must equal `origin/main` and the tree must be clean.
  - Clear the 8 open PRs; get CI and Release Candidate green on that `main` SHA.
  - Capture real SSH and AWS SSM smoke at that SHA: `LAZYSS_RELEASE_VERSION=v0.3.0 make live-smoke-evidence-template` (the Makefile defaults to `v0.1.0`), then validate.
  - Owner decides whether GCP/Azure smoke is required. Documented here as optional; `scripts/live_smoke_evidence.py` and release readiness do not check it. Making it required needs a Python schema change, not done in this brief.
  - Date the CHANGELOG `v0.3.0` heading when tagging (no date now, as in v0.2.0).
  - Not fixed here (code or script, out of scope); fix before tag or accept:
    - `internal/doctor/doctor.go`: when the azure `ssh` extension is missing, the detail says `az extension add --name ssh` but the printed fix says `run az login`.
    - `scripts/homebrew_formula.py` template `desc` still reads "Terminal machine cockpit for SSH and AWS SSM".
    - `v0.1.0` defaults remain in `Makefile` (`live-smoke-evidence-template`), `scripts/release-readiness.sh`, and commands in `docs/runbooks/release.md` and `homebrew.md`.
  - No tag, release, secret, branch protection, or tap change was made.
