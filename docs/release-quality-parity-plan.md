# LazySS Release and Quality Parity Plan

## Purpose

This plan upgrades Lazy Secure Shell from a working private alpha into a
release-quality CLI/TUI project comparable to the `lazyssh` and `lazyssm`
reference repos, with a private GitHub release path, Homebrew installation, and
stronger quality gates.

Current evidence before this plan:

- Private repo: `hamardikan/lazyss`.
- Local binary build works: `make build` produces `bin/lazyss`.
- GitHub fast CI on `main` is green, including format, vet, race tests, build,
  safe local smoke, lint, and govulncheck. Release-candidate CI covers the
  Linux/macOS/Windows amd64/arm64 build matrix and GoReleaser snapshot.
- Current code already has the intended DDD package layout, app services,
  adapters, TUI model, doctor command, and focused tests. Release hardening must
  preserve those boundaries instead of replacing them with packaging-only work.
- Reference directories `lazyssh/` and `lazyssm/` remain ignored and clean.

## Research Findings

### Reference Repos

`lazyssm` has the stronger quality baseline:

- `.goreleaser.yaml` with GoReleaser v2, `CGO_ENABLED=0`, archives, checksums,
  GitHub changelog, and linux/darwin/windows amd64/arm64 builds.
- CI split into format/vet/race coverage, `golangci-lint`, `govulncheck`, and
  push-only cross-platform build matrix.
- Release workflow uses `goreleaser/goreleaser-action@v7`, `fetch-depth: 0`,
  Go version from `go.mod`, and `GITHUB_TOKEN`.

`lazyssh` has the Homebrew publishing reference:

- `.goreleaser.yaml` includes a `brews` section publishing to a tap repo.
- Release workflow uses GoReleaser, but its workflow pins older Actions and a
  Go version lower than its current `go.mod`, so do not copy it literally.

### Official Tooling

GoReleaser’s current GitHub Actions guide recommends:

- `goreleaser/goreleaser-action@v7`
- `version: "~> v2"`
- `args: release --clean`
- `actions/checkout` with `fetch-depth: 0`
- `contents: write`
- `GITHUB_TOKEN` for same-repository GitHub Releases
- a separate PAT when publishing a Homebrew tap in another repository

GoReleaser’s current Homebrew publishing docs changed since older references:

- `homebrew_casks` exists since GoReleaser v2.10 and is the recommended current
  path.
- GoReleaser’s `brews`/Homebrew Formula support is marked deprecated in favor of
  `homebrew_casks`.
- Generated GoReleaser Homebrew formulae/casks target personal taps, not
  Homebrew core.
- Private GitHub releases used by Homebrew require special handling. For a
  private repo, Homebrew users need token-backed download support; GoReleaser’s
  docs describe a custom `CurlDownloadStrategy` for private GitHub repository
  casks.
- Since Homebrew 5.1.14, `HOMEBREW_GITHUB_API_TOKEN` is scrubbed from cask
  evaluation context. A private cask must resolve the token inside a custom
  download strategy rather than embedding it directly in the URL/header template.

Homebrew’s own docs recommend:

- Tap repository names start with `homebrew-`, e.g. `hamardikan/homebrew-tap`.
- `brew tap-new owner/homebrew-tap` creates the correct local structure.
- Formulae need a stable tagged version and should pass `brew audit`.

## Release Decision

Use **GoReleaser v2 with `homebrew_casks`** as the primary Homebrew route because
it matches current GoReleaser guidance.

Target install command after release:

```sh
brew install --cask hamardikan/tap/lazyss
```

If the required UX is exactly `brew install hamardikan/tap/lazyss` without
`--cask`, create a follow-up formula track. That track should be explicit
because GoReleaser’s formula support is currently documented as deprecated.

For private releases, the plan must support token-backed Homebrew download.
Recommended V1 release posture:

1. Keep `hamardikan/lazyss` private.
2. Run `./scripts/homebrew-readiness.sh` as a read-only Milestone 0 audit.
3. Create `hamardikan/homebrew-tap` private unless a public install path is
   desired.
4. Add `HOMEBREW_TAP_GITHUB_TOKEN` or `GH_PAT` to `hamardikan/lazyss` secrets
   with access to both repos.
5. Document `HOMEBREW_GITHUB_API_TOKEN` for local Homebrew installs from the
   private release.
6. If private cask installation cannot be made reliable, keep the source repo
   private but publish release assets publicly as an explicit release decision.

## Target State

### Files to Add or Replace

- `.goreleaser.yaml`
- `.golangci.yml`
- `.github/workflows/ci.yml`
- `.github/workflows/release.yml`
- `.github/ISSUE_TEMPLATE/bug_report.yml`
- `.github/ISSUE_TEMPLATE/feature_request.yml`
- `.github/pull_request_template.md`
- `.github/dependabot.yml`
- `.github/CODEOWNERS`
- `LICENSE`
- `CHANGELOG.md`
- `CONTRIBUTING.md`
- `SECURITY.md`
- `docs/runbooks/homebrew.md`
- `docs/runbooks/release.md`
- `docs/runbooks/smoke.md`
- `docs/runbooks/quality-gates.md`

### CI Quality Gates

CI should be split into fast PR feedback and heavier release-candidate proof.

Fast CI runs on pull requests and pushes to `main`:

- Format check: `gofmt -l .`
- Module drift check: `go mod tidy` followed by a clean `go.mod`/`go.sum` diff
- Vet: `go vet ./...`
- Race tests with coverage: `go test -race -coverprofile=coverage.out ./...`
- Coverage baseline check using `coverage.baseline`
- Coverage artifact upload
- Script tests for release helper scripts
- Linux build: `go build ./cmd/lazyss`
- Safe local binary/TUI smoke: `make smoke-local`
- `golangci-lint` using `.golangci.yml`, with the action pinned to
  `version: v2.12.2` like `lazyssm`
- `govulncheck`
- `ci-required` aggregate status for branch protection

Release-candidate CI runs on release-automation pull requests, relevant `main`
pushes, and manual dispatch:

- Cross-platform build matrix for linux/darwin/windows amd64/arm64
- GoReleaser snapshot validation
- Archive and checksum verification for the snapshot `dist/` directory
- Short-retention upload of snapshot `dist/` artifacts for review
- Homebrew readiness audit, with approval/external-state blockers reported
  without hiding local configuration failures
- `release-candidate-required` aggregate status

Superseded workflow runs should be cancelled automatically per pull request or
branch so CI does not waste time on stale commits.

Branch protection should require `ci-required`, not every component job name.
The component jobs remain visible for diagnosis, while the aggregate gives
GitHub branch protection a stable contract.

### Runtime Quality Gates

Packaging parity is not enough. Before a tag is cut, the locally built binary
and the release candidate binary must pass smoke checks for the real operator
loop:

- `lazyss --version` prints the tagged version.
- `lazyss doctor` reports local tool and AWS-readiness status without leaking
  credential material.
- `lazyss --source ssh` can read `~/.ssh/config` without mutating it.
- Direct SSH connection launch works for one known-good SSH host.
- `lazyss --source aws` preserves SSH inventory when AWS provider setup is
  missing or degraded.
- AWS SSM inventory and shell launch work for one known SSM-ready instance.
- Local state is written under the user config directory with mode `0600`.
- Failed connection attempts do not update last successful connection state.

Use `make live-smoke-evidence-template` to create the ignored local evidence
file for these checks. The helper writes the current commit and target version,
uses `0600` permissions, and leaves pass/fail fields false until the operator
has completed the real SSH and AWS SSM smoke checks.

The safe automated subset is `make smoke-local`. It builds the binary, checks
the version command, runs `lazyss doctor` with EC2 metadata disabled, starts the
TUI in a pseudo-terminal with a temporary SSH config, verifies the temp SSH row
renders, and confirms the temp SSH config was not mutated. Live direct SSH and
AWS SSM session launch remain release-candidate gates because they need approved
real targets and valid operator credentials.

### Release Gates

Release should be tag-driven:

```sh
git tag v0.1.0
git push origin v0.1.0
```

Release workflow should:

- run on `v[0-9]+.[0-9]+.[0-9]+` tags
- checkout with `fetch-depth: 0`
- use `actions/setup-go` with `go-version-file: go.mod`
- run `./scripts/release-readiness.sh` in tag mode before publishing artifacts
- run GoReleaser v2 with `release --clean`
- publish GitHub release archives, checksums, and changelog
- publish/update the Homebrew tap cask

Hosted release readiness requires approved secret names only:
`LAZYSS_LIVE_SMOKE_EVIDENCE_JSON` for live smoke evidence and
`LAZYSS_RELEASE_READINESS_GITHUB_TOKEN` for read-only GitHub readiness checks.
Do not store or print token values in docs, logs, state, or generated artifacts.

### GoReleaser Shape

The `.goreleaser.yaml` should include:

```yaml
version: 2
project_name: lazyss

before:
  hooks:
    - go mod tidy

builds:
  - main: ./cmd/lazyss
    binary: lazyss
    env:
      - CGO_ENABLED=0
    ldflags:
      - -s -w -X main.version={{.Version}}
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64

archives:
  - formats: [tar.gz]
    format_overrides:
      - goos: windows
        formats: [zip]

checksum:
  name_template: checksums.txt

changelog:
  use: github
  sort: asc

homebrew_casks:
  - name: lazyss
    ids:
      - lazyss
    binaries:
      - lazyss
    homepage: "https://github.com/hamardikan/lazyss"
    description: "Terminal machine cockpit for SSH and AWS SSM"
    repository:
      owner: hamardikan
      name: homebrew-tap
      branch: main
      token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"
    caveats: |
      LazySS reads ~/.ssh/config and uses the AWS CLI/session-manager-plugin for AWS SSM sessions.
```

If private release assets are used, add a custom private GitHub download
strategy to the cask as described in the official GoReleaser docs, and document
that users must export `HOMEBREW_GITHUB_API_TOKEN`.

### Lint Configuration

Start with the `lazyssm` linter profile rather than `lazyssh`’s stricter profile
because LazySS is younger and should not absorb a high-noise lint baseline on the
first release-hardening pass.

Minimum `.golangci.yml`:

```yaml
version: "2"
linters:
  enable:
    - errcheck
    - govet
    - ineffassign
    - staticcheck
    - unused
    - misspell
    - revive
```

If this exposes many findings, fix production code instead of disabling rules.
Only add exclusions for test helpers or clear false positives.

Use `golangci/golangci-lint-action@v9` with `version: v2.12.2` in CI. Do not
leave the linter version floating.

### DDD and TDD Parity Posture

LazySS already has the intended architecture boundary shape: `domain`, `ports`,
`app`, adapters, `tui`, and `doctor`. Release-quality work must preserve that
shape and avoid broad refactors unless a failing test or release gate proves the
need.

The strongest existing quality asset is the focused test suite. Do not replace
it with packaging-only confidence. Extend it only where release hardening changes
behavior, and keep `coverage.baseline` plus `docs/runbooks/quality-gates.md` in
sync. The first release uses a conservative total-coverage baseline gate: lower
only with an explicit PR rationale, and ratchet upward when meaningful coverage
is added.

## Implementation Plan

### Milestone 0: Private Homebrew Feasibility Spike

Goal: resolve the hardest release uncertainty before investing in tap docs or
release automation.

Tasks:

1. Confirm whether the target install must be private-source/private-assets or
   private-source/public-assets.
2. If private assets are required, prove the cask can download one private
   GitHub release asset from a clean Homebrew environment using a custom download
   strategy and `HOMEBREW_GITHUB_API_TOKEN`.
3. If private cask download is unreliable, document the decision to publish
   release assets publicly while keeping the source repository private.
4. Do not create repos, add tokens, or cut tags without explicit user approval.

Acceptance:

- A written decision exists in the plan/runbook: private assets with proven cask
  strategy, or public release assets with private source.
- No token values are printed or stored.
- The remaining release milestones know which Homebrew path they are building.

STOP gate: do not finalize `.goreleaser.yaml` Homebrew publishing or
`docs/runbooks/homebrew.md` until this milestone is resolved.

### Milestone 1: Release Metadata and Docs

Goal: make the project look installable and maintainable before changing release
automation.

Tasks:

1. Add a `LICENSE` decision. All-rights-reserved is acceptable while private,
   but the repo must be explicit before publishing binary artifacts.
2. Add `CHANGELOG.md` with an unreleased section and initial v0.1.0 target.
3. Add `CONTRIBUTING.md` with branch, test, commit, PR, and release flow.
4. Add `SECURITY.md` describing no-secret guarantees and private vulnerability
   reporting.
5. Add issue templates for bug reports and feature requests.
6. Update the PR template so every PR lists touched DDD boundary, tests, and
   verification output.
7. Add `CODEOWNERS` for the root owner.
8. Add Dependabot for Go modules and GitHub Actions.
9. Expand README with install modes:
   - `make build`
   - GitHub release archive
   - Homebrew cask
   - `go install github.com/hamardikan/lazyss/cmd/lazyss@latest` only as an
     authenticated developer path while the repo remains private
10. Draft `docs/runbooks/homebrew.md` only after Milestone 0 selects the asset
    strategy; include tap setup, secrets, private install token requirements,
    and `brew install --cask` validation.
11. Add `docs/runbooks/quality-gates.md` with local and hosted gates.

Acceptance:

- Docs mention no secret values.
- README no longer implies `go run` is the normal user install path.
- `git diff --check` passes.

### Milestone 2: Lint and Coverage Parity

Goal: match or exceed `lazyssm` quality gates.

Tasks:

1. Add `.golangci.yml` based on `lazyssm`.
2. Update `.github/workflows/ci.yml` to split jobs:
   - `test`
   - `lint`
   - `govulncheck`
   - `build`
3. Pin `golangci-lint-action` to `version: v2.12.2`.
4. Add coverage profile generation, baseline verification, and upload.
5. Capture the current coverage total in `coverage.baseline` and
   `docs/runbooks/quality-gates.md`.
6. Add local `make cover`.
7. Add local `make lint` if a local `golangci-lint` binary is available, but do
   not require local installs to run normal `make check`.

Acceptance:

```sh
gofmt -l .
go vet ./...
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -1
go build ./cmd/lazyss
```

Hosted CI must pass after pushing.

### Milestone 3: GoReleaser Snapshot

Goal: make release artifact generation reproducible before publishing a real
tag.

Tasks:

1. Add `.goreleaser.yaml` with builds, archives, checksums, changelog, and
   `homebrew_casks` configured with `skip_upload: true` for local snapshot
   validation first.
2. Replace manual `.github/workflows/release.yml` with GoReleaser action.
3. Add a PR/CI snapshot check:

   ```sh
   goreleaser release --clean --snapshot --skip=publish
   ```

4. Ensure dist output includes darwin/linux/windows amd64/arm64 archives and
   checksums.

Acceptance:

```sh
goreleaser check
goreleaser release --clean --snapshot --skip=publish
```

If GoReleaser is unavailable locally, install it through Homebrew or run the
equivalent GitHub Action and record the hosted evidence.

### Milestone 4: Homebrew Tap

Goal: publish a private Homebrew install path.

Prerequisite: Milestone 0 must have selected and proven the private/public asset
strategy.

Tasks:

1. Check whether `hamardikan/homebrew-tap` exists:

   ```sh
   gh repo view hamardikan/homebrew-tap
   ```

2. If absent, create it private:

   ```sh
   gh repo create hamardikan/homebrew-tap --private
   ```

3. Initialize tap structure locally if needed:

   ```sh
   brew tap-new hamardikan/homebrew-tap
   ```

4. Add repository secret to `hamardikan/lazyss`:

   - Name: `HOMEBREW_TAP_GITHUB_TOKEN`
   - Scope: content write access to `hamardikan/homebrew-tap`
   - Do not print token value in chat, logs, docs, or commands.

5. Switch GoReleaser `homebrew_casks.skip_upload` to false or remove it.
6. Add private GitHub cask download strategy if `hamardikan/lazyss` remains
   private. Do not place `HOMEBREW_GITHUB_API_TOKEN` directly in a cask URL or
   header template.
7. Release a dry-run tag in a branch or use `goreleaser release --snapshot`
   until the cask file shape is correct.

Acceptance:

- Tap repo receives `Casks/lazyss.rb`.
- The generated cask references the correct release asset names and checksums.
- Private install docs are clear about `HOMEBREW_GITHUB_API_TOKEN`.

### Milestone 5: v0.1.0 Release

Goal: cut the first installable release after smoke tests.

Prerequisites:

- Local quality gates pass.
- Hosted CI on `main` passes.
- GoReleaser snapshot passes.
- Real smoke tests pass for:
  - direct SSH host from `~/.ssh/config`
  - `lazyss doctor`
  - missing AWS credentials
  - one AWS SSM-ready instance
  - no secrets in state/log output

Release:

```sh
git tag v0.1.0
git push origin v0.1.0
gh run watch --repo hamardikan/lazyss
```

Acceptance:

- GitHub Release `v0.1.0` exists.
- Release contains archives for linux/darwin/windows amd64/arm64 plus
  `checksums.txt`.
- `DIST=/path/to/release-artifacts make release-artifacts-verify` passes.
- Homebrew tap is updated.
- Install works:

  ```sh
  brew install --cask hamardikan/tap/lazyss
  lazyss --version
  lazyss doctor
  ```

### Milestone 6: Branch Protection and Maintenance

Goal: keep release quality stable after v0.1.0.

Tasks:

1. Protect `main`:
   - require PRs
   - require `ci-required`
   - require branch up to date
   - disallow force-push
2. Validate the read-only target state with:

   ```sh
   ./scripts/branch-protection-readiness.sh
   ```

3. Add release checklist to `docs/runbooks/release.md`.
4. Add issue labels and milestone `v0.2.0`.

Acceptance:

- `main` cannot be pushed directly.
- `./scripts/branch-protection-readiness.sh` exits `0`.
- Dependabot opens update PRs when dependencies drift.
- Release checklist is actionable from a clean checkout.

## Risks and Decisions

### Private Homebrew Is More Complex Than Public Homebrew

Private release assets are not directly installable by unauthenticated Homebrew.
The plan must either:

1. keep both release and tap private and require token-backed download, or
2. make release assets public.

Default: keep private for now and document token-backed install.

### Formula vs Cask

`lazyssh` uses GoReleaser `brews`, but current GoReleaser docs mark that path
deprecated. LazySS should use `homebrew_casks` first. If exact `brew install`
without `--cask` is important, create a separate formula plan with an explicit
acceptance of the deprecated GoReleaser formula path or maintain the formula
manually.

### Code Signing

Homebrew cask docs strongly prefer signed/notarized macOS binaries. LazySS V1
can ship unsigned private binaries, but the runbook must document the expected
macOS quarantine behavior and how to handle it. Do not add automatic `xattr`
bypass without an explicit security decision.

## Goal Mode Prompt

````md
Implement LazySS release and quality parity in `/Users/hamardikan-mac/repos/personal-projects/lazyss`.

Objective:
Upgrade Lazy Secure Shell from private alpha to a release-quality CLI/TUI project comparable to the `lazyssh` and `lazyssm` reference repos, with stronger CI, GoReleaser artifact publishing, private Homebrew install support, release docs, and a verified v0.1.0 path.

Current baseline:
- Repo: `hamardikan/lazyss` private.
- `main` CI currently passes.
- Local `make build` produces `bin/lazyss`.
- The code already has the DDD package layout, app services, adapters, TUI, doctor, and focused tests. Preserve these boundaries; do not do broad architecture refactors unless a failing gate proves the need.
- Reference repos `lazyssh/` and `lazyssm/` must stay ignored and unmodified.

Hard constraints:
- Do not print or store tokens, private keys, AWS credentials, SSO cache contents, or environment dumps.
- Do not mutate `lazyssh/` or `lazyssm/`.
- Keep LazySS repo private unless the user explicitly says otherwise.
- Prefer current GoReleaser v2 `homebrew_casks` over deprecated `brews` unless the user explicitly chooses formula-only UX.
- Do not tag `v0.1.0` until local gates, hosted CI, GoReleaser snapshot, and real smoke tests pass.
- Do not create or use a Homebrew tap token unless the user provides/approves the secret setup.
- Ask before outward irreversible/admin actions: creating `hamardikan/homebrew-tap`, adding repository secrets, enabling branch protection, cutting the first release tag, or making release assets public.
- Work through PR-style commits; do not rely on direct `main` pushes after branch protection is enabled.

Milestones:
0. Resolve private Homebrew feasibility:
   - Decide private-source/private-assets vs private-source/public-assets.
   - If private assets are required, prove a cask can download a private GitHub release asset from a clean Homebrew environment using a custom download strategy and `HOMEBREW_GITHUB_API_TOKEN`.
   - Do not write final Homebrew publishing config until this is resolved.
   - Record the decision in the Homebrew runbook or ADR.
1. Add release metadata/docs:
   - `LICENSE` decision
   - `CHANGELOG.md`
   - `CONTRIBUTING.md`
   - `SECURITY.md`
   - issue templates
   - PR template requiring DDD boundary, tests, and verification output
   - `CODEOWNERS`
   - Dependabot for Go modules and GitHub Actions
   - expanded README install docs
   - `docs/runbooks/homebrew.md` after Milestone 0 selects the asset strategy
   - `docs/runbooks/quality-gates.md`
2. Add lint/coverage parity:
   - `.golangci.yml`
   - split CI jobs for test/lint/govulncheck/build
   - pin `golangci-lint-action` to `version: v2.12.2`
   - race coverage profile and upload
   - capture coverage baseline in `coverage.baseline` and
     `docs/runbooks/quality-gates.md`
   - `make cover`
3. Add GoReleaser:
   - `.goreleaser.yaml`
   - GoReleaser release workflow
   - `goreleaser check` before trusting config fields
   - snapshot validation with `goreleaser release --clean --snapshot --skip=publish`
4. Prepare Homebrew tap:
   - only proceed after Milestone 0 is resolved
   - check/create `hamardikan/homebrew-tap`
   - configure `homebrew_casks`
   - document private install token requirements
   - validate generated cask
5. Cut v0.1.0 only after:
   - local gates pass
   - GitHub CI passes
   - GoReleaser snapshot passes
   - real SSH and AWS SSM smoke tests pass
6. Add branch protection and maintenance:
   - protected `main`
   - release checklist updates

Required local gates:
```sh
gofmt -l .
go vet ./...
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -1
go build ./cmd/lazyss
```

Required hosted gates:
- GitHub CI test job passes.
- GitHub CI lint job passes.
- GitHub CI govulncheck job passes.
- GitHub CI build matrix passes for linux/darwin/windows amd64/arm64.
- GoReleaser snapshot check passes.

Runtime smoke gates before tagging:
- `lazyss --version` prints the release version.
- `lazyss doctor` runs without leaking credentials.
- `lazyss --source ssh` reads SSH config without mutating it.
- one direct SSH launch works.
- AWS degraded setup does not hide SSH inventory.
- one AWS SSM-ready instance can be inventoried and launched.
- state file permissions and failed connection history remain correct.

Deliverables:
- Committed release-quality config/docs, pushed through the agreed branch/PR flow.
- Private Homebrew tap decision and plan, or created tap only after explicit approval/secrets.
- Release runbook with exact commands.
- Clear statement of whether `v0.1.0` was tagged or why it was intentionally held back.
````

## Sources

- GoReleaser GitHub Actions docs:
  `https://goreleaser.com/customization/ci/actions/`
- GoReleaser GitHub token docs:
  `https://goreleaser.com/customization/publish/scm/github/`
- GoReleaser Homebrew Casks docs:
  `https://goreleaser.com/customization/publish/homebrew_casks/`
- GoReleaser deprecated Homebrew Formulas docs:
  `https://goreleaser.com/customization/publish/homebrew_formulas/`
- Homebrew tap docs:
  `https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap`
- Homebrew formula cookbook:
  `https://docs.brew.sh/Formula-Cookbook`
