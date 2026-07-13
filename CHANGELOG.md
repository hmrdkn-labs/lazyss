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
- Cockpit key help overlay (`?`) and a footer key registry that is the single
  source for dispatch, footer, and help wording, including `m` method cycling
  and `c` copy connect command.
- Detail panel toggle (`h`), overlay editor (`e`) for note, tags, and preferred
  method, and a full session and health history view (`v`).
- Guarded in-cockpit SSH config cleanup (`C`) with dry-run plan, protected SCM
  identity hosts, duplicate-target delete candidates, and confirm-to-write.
- Streaming bounded health checks (`g` selected, `G` all visible) and a
  windowed AWS profile picker.
- Proportional list columns with a visible `›` cursor, shape-based health
  glyphs, aligned split panels, and a reserved status line.
- tmux-driven cockpit smoke test (`make smoke-tui`) with no AWS dependency.

### Fixed

- Search (`/`) and filter (`f`) text entry dropped the space key under Bubble
  Tea v2; typed text now uses the key's printable text.

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
