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

The private download strategy must keep authentication in runtime strategy state
and request headers only. It must not return or print a URL containing
`HOMEBREW_GITHUB_API_TOKEN`.

The release-candidate snapshot gate verifies the generated
`homebrew/Casks/lazyss.rb` file, not only `.goreleaser.yaml`. It checks that the
cask uses `GitHubPrivateRepositoryReleaseDownloadStrategy`, reads
`HOMEBREW_GITHUB_API_TOKEN` at download time, avoids token-bearing URLs, and
keeps its darwin/linux archive checksums aligned with `checksums.txt`.

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
./scripts/homebrew-readiness.sh
```

The audit is read-only. It checks local tools, `.goreleaser.yaml`, private repo
visibility, tap repository visibility, tap publishing secret name presence, and
local tap state. It does not create repositories, add secrets, cut tags, publish
releases, or print token values.

Hosted release preflight uses:

```sh
LAZYSS_HOMEBREW_READINESS_MODE=hosted ./scripts/homebrew-readiness.sh
```

Hosted mode skips only local `brew tap` state because the GitHub release runner
is not the operator Homebrew machine. It still checks tracked release
configuration, GitHub repository visibility, tap visibility, and secret names
when the token has permission to read them.

In the release workflow, hosted readiness uses
`LAZYSS_RELEASE_READINESS_GITHUB_TOKEN` so the audit can read the private tap and
the LazySS repository's readiness state before GoReleaser receives
`HOMEBREW_TAP_GITHUB_TOKEN` for publishing.

Exit codes:

- `0`: Homebrew readiness prerequisites are satisfied.
- `1`: local release config or tool setup has a failure that can be fixed before
  approval.
- `2`: only approval or external-state blockers remain.

Before approval, `exit 2` is expected when `hamardikan/homebrew-tap`,
`HOMEBREW_TAP_GITHUB_TOKEN`, or the local `hamardikan/tap` tap are missing.
The `make homebrew-readiness` target is a convenience wrapper for local human
use, but call the script directly when exact exit-code handling matters.

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

The release workflow references `HOMEBREW_TAP_GITHUB_TOKEN` by secret name only.
It must not print token values.

Before approving a tag, download the `goreleaser-snapshot-<sha>` artifact from
the release-candidate workflow and run:

```sh
DIST=/path/to/downloaded/dist make release-artifacts-verify
```

This validates the generated private cask shape without creating the tap,
adding secrets, publishing a release, or requiring a real Homebrew install.

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

Before requesting release approval, create the ignored local evidence file:

```sh
make homebrew-private-evidence-template
```

After the private install test passes, edit only the non-secret booleans and
labels in `homebrew-private-evidence.json`, then validate it:

```sh
python3 scripts/homebrew_private_evidence.py validate \
  --file homebrew-private-evidence.json \
  --target-version v0.1.0 \
  --commit "$(git rev-parse HEAD)"
```

The file must remain local or be attached only as a private release artifact. It
must not contain token values, authorization headers, private release asset API
URLs, Homebrew debug logs with token material, AWS credentials, SSH keys, or
environment dumps.

Release readiness requires this proof through:

```sh
LAZYSS_HOMEBREW_PRIVATE_EVIDENCE=homebrew-private-evidence.json \
./scripts/release-readiness.sh
```

## Fallback Decision

If private cask installation cannot be made reliable, stop and write a follow-up
ADR. The fallback is to keep `hamardikan/lazyss` private while publishing release
assets through an explicitly approved public artifact path.
