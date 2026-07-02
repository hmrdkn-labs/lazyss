# LazySS Homebrew Runbook

LazySS uses a public Homebrew formula as the primary CLI install path:

```sh
brew install --formula hmrdkn-labs/tap/lazyss
```

## Current Decision

ADR 0002 selects public GitHub release assets and a public Homebrew formula.
GoReleaser builds and publishes the release archives. LazySS release automation
then generates `homebrew/Formula/lazyss.rb` from the snapshot `dist/` directory
and publishes `Formula/lazyss.rb` to `hmrdkn-labs/homebrew-tap`.

The formula must use public release URLs, release checksums, `bin.install
"lazyss"`, and a `lazyss --version` test. It must not contain token values,
authorization headers, private GitHub API asset URLs, or custom private
download strategies.

LazySS does not publish a cask for V1. Reconsider a cask only after Developer ID
signing and notarization are available.

## Approval Gates

Ask for explicit owner approval before any of these actions:

- transferring the repository to `hmrdkn-labs`
- changing repository visibility to public
- creating `hmrdkn-labs/homebrew-tap`
- adding `HOMEBREW_TAP_GITHUB_TOKEN` or any tap publishing secret
- cutting the first release tag
- enabling or changing branch protection

## Read-Only Readiness Audit

Run the Homebrew readiness audit before requesting release approval:

```sh
./scripts/homebrew-readiness.sh
```

The audit is read-only. It checks local tools, `.goreleaser.yaml`, public
formula shape, source repository visibility, tap repository visibility, tap
publishing secret name presence, and local tap state. It does not create
repositories, add secrets, cut tags, publish releases, or print token values.

Hosted release preflight uses:

```sh
LAZYSS_HOMEBREW_READINESS_MODE=hosted ./scripts/homebrew-readiness.sh
```

Hosted mode skips only local `brew tap` state because the GitHub release runner
is not the operator Homebrew machine. It still checks tracked release
configuration, GitHub repository visibility, tap visibility, and secret names
when the token has permission to read them.

Exit codes:

- `0`: Homebrew readiness prerequisites are satisfied.
- `1`: local release config or tool setup has a failure that can be fixed before
  approval.
- `2`: only approval or external-state blockers remain.

Before public launch, `exit 2` is expected when `hmrdkn-labs/lazyss` is still
private, `hmrdkn-labs/homebrew-tap` is missing, `HOMEBREW_TAP_GITHUB_TOKEN` is
missing, or the local `hmrdkn-labs/tap` tap is not tapped.

## Tap Setup

Check whether the tap exists:

```sh
gh repo view hmrdkn-labs/homebrew-tap
```

If the owner approves creating it:

```sh
gh repo create hmrdkn-labs/homebrew-tap --public --description "Homebrew tap for hmrdkn-labs tools"
brew tap hmrdkn-labs/tap
```

The tap repository should contain formulae under `Formula/`.

## Release Publishing

The release workflow must keep:

```sh
python3 scripts/homebrew_formula.py generate \
  --dist dist \
  --version "$TAG" \
  --output homebrew-tap/Formula/lazyss.rb
```

The release workflow references `HOMEBREW_TAP_GITHUB_TOKEN` by secret name only
when checking out `hmrdkn-labs/homebrew-tap`. The token must have content write
access to the tap repository. It must not be printed in logs, docs, evidence
files, or release notes.

Before approving a tag, download the `goreleaser-snapshot-<sha>` artifact from
the release-candidate workflow and run:

```sh
DIST=/path/to/downloaded/dist make release-artifacts-verify
```

This validates the expected archives, checksums, and generated public formula
without creating the tap, adding secrets, publishing a release, or requiring a
real Homebrew install.

## Install Test

After the release workflow publishes the formula:

```sh
brew update
brew uninstall --formula lazyss || true
brew install --formula hmrdkn-labs/tap/lazyss
lazyss --version
lazyss doctor
```

Expected result:

- Homebrew installs the formula from the public tap.
- `lazyss --version` prints the release version.
- `lazyss doctor` reports local readiness without leaking credentials.

Record only non-secret command output in the release issue.
