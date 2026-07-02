# ADR 0003: License Posture

Status: amended for public launch

Date: 2026-07-02

## Context

LazySS is moving from a private personal repository to the public
`hmrdkn-labs/lazyss` repository. A public repository needs an explicit license
so users, contributors, package managers, and mirrors understand the reuse
terms.

## Decision

LazySS uses the MIT License for V1.

The root `LICENSE` file is the authoritative license text. The LazySS Homebrew
formula generator also declares `license "MIT"` in the generated formula.

## Consequences

- Users can use, copy, modify, and distribute LazySS under MIT terms.
- Contributors can evaluate the project without ambiguity about reuse rights.
- Future license changes require a new ADR and maintainer approval.
