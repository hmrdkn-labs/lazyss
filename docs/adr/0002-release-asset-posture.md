# ADR 0002: Release Asset and Homebrew Posture

Status: amended after v0.1.0 publish

Date: 2026-07-02

## Context

LazySS is a private repository and should remain private unless the owner
explicitly approves a visibility change. The pre-release target install path was
a Homebrew cask based on GoReleaser cask publishing guidance. The `v0.1.0`
post-publish proof showed that unsigned cask binaries can retain macOS
quarantine and be blocked by Gatekeeper with a message that macOS cannot verify
the binary is free of malware.

Homebrew formula installation of the same private release archive runs without
that quarantine block. The private tap still needs token-backed downloads
because release assets remain private.

## Decision

V1 keeps both source and release assets private by default.

The primary Homebrew path is:

```sh
brew install --formula hamardikan/tap/lazyss
```

The tap package must use a private GitHub download strategy that reads
`HOMEBREW_GITHUB_API_TOKEN` at download time. LazySS must not embed token values
in package URLs, headers, docs, CI logs, local state, or release metadata.

A cask may remain as a secondary generated artifact, but it is not the primary
operator install path until binaries are Developer ID signed and notarized.

## Proof Protocol

The private Homebrew download cannot be fully proven without approved external
state: a release asset, a tap repository, and an operator token. For the first
release, the install proof is a post-publish gate because `v0.1.0` assets and
the tap package do not exist until publishing completes:

1. Build a release candidate with GoReleaser snapshot and verify generated
   archive names, checksums, and private download strategy.
2. Publish the private release assets and tap package.
3. Export `HOMEBREW_GITHUB_API_TOKEN` in the operator shell without printing it.
4. After the real release publishes, install from a clean Homebrew environment:

   ```sh
   brew install --formula hamardikan/tap/lazyss
   lazyss --version
   lazyss doctor
   ```

5. Confirm no token value appears in shell history, CI logs, generated tap
   files, state files, or docs.

## Consequences

- Homebrew publishing remains gated on explicit approval for the tap repository
  and token, then runs as part of the release path.
- Formula install is the supported private macOS operator path for unsigned CLI
  binaries.
- Cask install remains blocked by macOS Gatekeeper until LazySS has Developer ID
  signing and notarization or the operator manually removes quarantine.
