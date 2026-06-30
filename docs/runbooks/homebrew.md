# LazySS Homebrew Runbook

LazySS uses a Homebrew cask path for V1:

```sh
brew install --cask hamardikan/tap/lazyss
```

## Current Decision

ADR 0002 selects private source and private release assets by default. The
generated cask must use a custom `CurlDownloadStrategy` and read
`HOMEBREW_GITHUB_API_TOKEN` at download time.

Do not place token values in cask `url.template`, cask headers, docs, CI logs,
release metadata, or local state.

## Approval Gates

Ask for explicit owner approval before any of these actions:

- creating `hamardikan/homebrew-tap`
- adding `HOMEBREW_TAP_GITHUB_TOKEN` or any tap publishing secret
- cutting the first release tag
- making release assets public
- enabling branch protection

## Read-Only Readiness Audit

Run the Homebrew readiness audit before requesting owner approval:

```sh
make homebrew-readiness
```

The audit is read-only. It checks local tools, `.goreleaser.yaml`, private repo
visibility, tap repository visibility, tap publishing secret name presence, and
local tap state. It does not create repositories, add secrets, cut tags, publish
releases, or print token values.

Exit codes:

- `0`: Homebrew readiness prerequisites are satisfied.
- `1`: local release config or tool setup has a failure that can be fixed before
  approval.
- `2`: only approval or external-state blockers remain.

Before approval, `exit 2` is expected when `hamardikan/homebrew-tap`,
`HOMEBREW_TAP_GITHUB_TOKEN`, or the local `hamardikan/tap` tap are missing.

## Tap Setup

Check whether the tap exists:

```sh
gh repo view hamardikan/homebrew-tap
```

If the owner approves creating it:

```sh
gh repo create hamardikan/homebrew-tap --private
brew tap-new hamardikan/homebrew-tap
```

The tap repository should contain casks under `Casks/`.

## Release Publishing

Before tap approval, `.goreleaser.yaml` must keep:

```yaml
homebrew_casks:
  - name: lazyss
    skip_upload: true
```

After tap approval, remove `skip_upload: true` and configure the tap repository
with a token that has content write access to `hamardikan/homebrew-tap`.

The release workflow must not print token values.

## Private Install Test

The operator shell must have a GitHub token exported for Homebrew. Do not print
the token value.

```sh
brew uninstall --cask lazyss || true
brew install --cask hamardikan/tap/lazyss
lazyss --version
lazyss doctor
```

Expected result:

- Homebrew installs the cask without exposing token material.
- `lazyss --version` prints the release version.
- `lazyss doctor` reports local readiness without leaking credentials.

## Fallback Decision

If private cask installation cannot be made reliable, stop and write a follow-up
ADR. The fallback is to keep `hamardikan/lazyss` private while publishing release
assets through an explicitly approved public artifact path.
