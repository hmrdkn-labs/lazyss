# ADR 0002: Release Asset and Homebrew Posture

Status: accepted for pre-release implementation

Date: 2026-06-30

## Context

LazySS is a private repository and should remain private unless the owner
explicitly approves a visibility change. The target install path is Homebrew
cask based on current GoReleaser guidance. GoReleaser formula support exists in
older reference repos, but current GoReleaser documentation marks formula
publishing as deprecated in favor of `homebrew_casks`.

Private GitHub release assets are not directly usable by unauthenticated
Homebrew installs. Current GoReleaser cask documentation supports private
repositories through a cask `custom_block` that defines a `CurlDownloadStrategy`
subclass and references that class from `url.using`. This is required because
Homebrew 5.1.14 scrubs sensitive environment variables during cask evaluation,
including `HOMEBREW_GITHUB_API_TOKEN`; token lookup must happen at download
time.

## Decision

V1 keeps both source and release assets private by default.

The Homebrew path is:

```sh
brew install --cask hamardikan/tap/lazyss
```

The generated cask must use a private GitHub download strategy that reads
`HOMEBREW_GITHUB_API_TOKEN` at download time. LazySS must not embed tokens in
cask URLs, headers, docs, CI logs, local state, or release metadata.

Until the owner approves the tap repository, tap token, and first release tag,
GoReleaser must keep Homebrew publishing in dry-run mode with `skip_upload:
true`. This allows snapshot validation and generated cask review without
mutating a tap repository.

## Proof Protocol

The private cask download cannot be fully proven without approved external
state: a release asset, a tap repository, and an operator token. The proof must
be run before `v0.1.0` is tagged:

1. Build a release candidate with GoReleaser snapshot or a temporary approved
   tag.
2. Generate `Casks/lazyss.rb` with the private download strategy.
3. Export `HOMEBREW_GITHUB_API_TOKEN` in the operator shell without printing it.
4. Install from a clean Homebrew environment:

   ```sh
   brew install --cask hamardikan/tap/lazyss
   lazyss --version
   lazyss doctor
   ```

5. Confirm no token value appears in shell history, CI logs, generated cask,
   state files, or docs.

## Consequences

- Release automation can be validated now with `skip_upload: true`.
- Homebrew publishing remains gated on explicit approval for the tap repository
  and token.
- If private asset installation fails during the proof protocol, the fallback is
  a new ADR deciding whether release assets may be public while the source repo
  remains private.
