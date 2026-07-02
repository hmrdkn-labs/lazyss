# ADR 0003: License Posture

## Status

Accepted.

## Context

LazySS is currently developed as a private personal project in
`hamardikan/lazyss`. The V1 release path is private GitHub release artifacts
and a private Homebrew formula flow. The project includes cloud and operator
access tooling where the first release must prioritize controlled distribution,
security review, and owner-approved release steps.

## Decision

LazySS uses an all-rights-reserved license for V1.

The root `LICENSE` file grants no copy, modification, distribution,
sublicensing, or usage rights without explicit written permission from the
copyright holder.

## Consequences

- The repository can remain private without implying an open-source grant.
- Release artifacts and Homebrew installation remain private unless the owner
  explicitly approves a visibility or licensing change.
- External reuse, redistribution, forks, package mirrors, public taps, and
  public release assets require an explicit future decision.
- If LazySS becomes public or collaborative, a new ADR must choose an open-source
  or source-available license before publishing broader installation paths.
