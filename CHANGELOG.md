# Changelog

All notable LazySS changes are recorded here.

## Unreleased

### Added

- Release-quality parity plan and private Homebrew decision record.
- Homebrew cask runbook for private release assets.
- Fast CI with format, module drift, vet, race coverage, script tests, build,
  smoke-local, pinned GolangCI-Lint, and pinned `govulncheck`.
- Release-candidate workflow for cross-platform builds, GoReleaser snapshot
  validation, archive/checksum verification, generated cask verification, and
  host archive smoke execution.
- Read-only release readiness, branch protection readiness, Homebrew readiness,
  live smoke evidence, private Homebrew evidence, and release approval handoff
  tooling.
- Branch protection, SDLC, smoke, security, quality gate, release, and Homebrew
  runbooks.
- Private all-rights-reserved license posture.

## v0.1.0

Target: first installable private release.

Planned contents:

- SSH config inventory.
- AWS SSM and EC2 inventory.
- Direct SSH and AWS SSM session launch.
- Manual health checks.
- Local state for pins, tags, notes, preferred method, health, and history.
- Bubble Tea cockpit and `lazyss doctor`.
- GoReleaser archives, checksums, and Homebrew cask validation.
