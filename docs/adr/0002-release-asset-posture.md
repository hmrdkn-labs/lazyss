# ADR 0002: Release Asset and Homebrew Posture

Status: amended for public launch

Date: 2026-07-02

## Context

LazySS started with a private repository and a token-backed Homebrew cask plan.
The first macOS install proof showed that unsigned cask binaries can retain
quarantine and be blocked by Gatekeeper. Formula installs avoid that cask
quarantine path and fit a terminal CLI better.

The project is moving to the public `hmrdkn-labs/lazyss` repository, so release
assets and Homebrew package metadata should be public. Private GitHub release
download strategies are no longer appropriate.

## Decision

LazySS publishes public GitHub release assets and a public Homebrew formula.

The primary install path is:

```sh
brew install --formula hmrdkn-labs/tap/lazyss
```

GoReleaser builds and publishes the release archives. LazySS release automation
then generates `Formula/lazyss.rb` from the GoReleaser `dist/` output and
pushes it to `hmrdkn-labs/homebrew-tap`. The formula uses public release URLs,
release checksums, `bin.install "lazyss"`, and a `lazyss --version` test.

A cask is not published for V1. A cask can be reconsidered after Developer ID
signing and notarization are in place.

## Proof Protocol

Before tagging:

1. Run the local and hosted release-candidate gates.
2. Verify the generated `homebrew/Formula/lazyss.rb` from the release-candidate
   snapshot.
3. Confirm the tap repository exists and the release workflow has a tap write
   secret named `HOMEBREW_TAP_GITHUB_TOKEN`.
4. Confirm real SSH and AWS SSM smoke evidence has been captured.

After publishing:

```sh
brew update
brew install --formula hmrdkn-labs/tap/lazyss
lazyss --version
lazyss doctor
```

## Consequences

- Public users can install without `GOPRIVATE` or a release-download token.
- The release workflow still needs a GitHub token with write access to the tap
  repository.
- The generated formula must not contain credentials, private API URLs, or
  custom token download strategies.
