# LazySS Public Release Quality Parity Plan

This plan tracks the work required for Lazy Secure Shell to reach public
release quality in `hmrdkn-labs/lazyss`.

## Target

- Public source repository: `hmrdkn-labs/lazyss`.
- Public Homebrew tap: `hmrdkn-labs/homebrew-tap`.
- License: MIT.
- Primary install path: `brew install --formula hmrdkn-labs/tap/lazyss`.
- Release assets: public GitHub release archives plus `checksums.txt`.
- Packaging: GoReleaser archives plus LazySS-generated `Formula/lazyss.rb`.

## Product Quality Bar

LazySS should feel like an operator cockpit, not only a proof-of-concept list:

- SSH inventory hides SCM identity aliases and retired entries by default.
- AWS SSM inventory supports profile selection, SSO login handoff, tag-aware
  filtering, and degraded-provider banners without hiding SSH inventory.
- Terminal handoff exits the Bubble Tea screen before starting SSH or SSM so
  the remote shell owns the operator terminal cleanly.
- Health checks use explicit labels such as `tcp host:port`, `ssm Online`, and
  EC2 state.
- Local state preserves pins, hidden entries, preferred method, health, and
  session history with `0600` permissions.

## Release Quality Bar

- `gofmt -l .`
- `go vet ./...`
- `go test -race -coverprofile=coverage.out ./...`
- `python3 -m unittest discover -s scripts -p '*_test.py'`
- `go build ./cmd/lazyss`
- `make smoke-local`
- `make build-matrix`
- `make release-snapshot`
- `DIST=dist make release-artifacts-verify`
- `./scripts/homebrew-readiness.sh`
- `LAZYSS_LIVE_SMOKE_EVIDENCE=live-smoke-evidence.json ./scripts/release-readiness.sh`

The release-candidate workflow mirrors the packaging gates in GitHub Actions.
It verifies cross-platform builds, GoReleaser snapshot output, generated public
formula content, checksums, archive contents, and host-binary execution.

## Homebrew Requirements

The release workflow must keep the formula generation and tap update steps:

```sh
python3 scripts/homebrew_formula.py generate \
  --dist dist \
  --version "$TAG" \
  --output homebrew-tap/Formula/lazyss.rb
```

Generated formula verification must confirm:

- `class Lazyss < Formula`
- `homepage "https://github.com/hmrdkn-labs/lazyss"`
- `license "MIT"`
- `bin.install "lazyss"`
- `system "#{bin}/lazyss", "--version"`
- darwin/linux archive URLs and checksums match `checksums.txt`
- no credential-like values or private download strategy fragments appear

## Public Launch Steps

1. Merge public-launch config and documentation to `main`.
2. Transfer `hamardikan/lazyss` to `hmrdkn-labs/lazyss`.
3. Set repository visibility to public.
4. Create `hmrdkn-labs/homebrew-tap` as a public repository.
5. Add `HOMEBREW_TAP_GITHUB_TOKEN` to `hmrdkn-labs/lazyss` with write access
   to the tap repository.
6. Add `LAZYSS_RELEASE_READINESS_GITHUB_TOKEN` so hosted tag readiness can read
   branch protection, workflows, repository state, secret names, and the tap.
7. Add `LAZYSS_LIVE_SMOKE_EVIDENCE_JSON` after real SSH and AWS SSM smoke
   evidence validates.
8. Run release readiness from clean `main`.
9. Tag `v0.1.0` only after readiness exits `0`.
10. Verify published archives, checksums, GitHub release metadata, Homebrew
    formula publication, `brew install`, `lazyss --version`, and `lazyss doctor`.

## Follow-Up Quality Work

- Improve the onboarding screen so first-run setup exposes source, profile,
  login, filter, and hide controls clearly.
- Expand structured filters from provider/source and tag filters into saved
  filter presets.
- Add provider-portable design notes for future GCP IAP, Azure Bastion, OCI
  Bastion, and Alibaba Cloud Session Manager adapters.
- Add signed/notarized cask support only after Developer ID signing and
  notarization are available.
