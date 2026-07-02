# Changelog

All notable LazySS changes are recorded here.

## Unreleased

### Added

- Public release-quality parity plan and Homebrew formula decision record.
- Homebrew formula runbook for public release assets.
- Fast CI with format, module drift, vet, race coverage, script tests, build,
  smoke-local, pinned GolangCI-Lint, and pinned `govulncheck`.
- Release-candidate workflow for cross-platform builds, GoReleaser snapshot
  validation, archive/checksum verification, generated formula verification,
  and host archive smoke execution.
- Read-only release readiness, branch protection readiness, Homebrew readiness,
  live smoke evidence, and release approval handoff tooling.
- Branch protection, SDLC, smoke, security, quality gate, release, and Homebrew
  runbooks.
- MIT license posture for public launch.

## v0.1.0

Target: first installable public release.

Planned contents:

- SSH config inventory.
- AWS SSM and EC2 inventory.
- Direct SSH and AWS SSM session launch.
- Manual health checks.
- Local state for pins, tags, notes, preferred method, health, and history.
- Bubble Tea cockpit and `lazyss doctor`.
- GoReleaser archives, checksums, and Homebrew formula validation.
