# LazySS SDLC Pipeline

LazySS uses a staged pipeline so day-to-day PR feedback stays fast while release
proof remains strict.

## Stages

| Stage | Trigger | Purpose | Required proof |
| --- | --- | --- | --- |
| Developer local gate | before commit or PR | catch basic breakage before CI | `make check` |
| Fast PR gate | pull request and `main` push | merge safety and branch protection | `ci-required` |
| Release candidate | release-relevant PR, `main` push, label, or manual dispatch | artifact and install-path proof | `release-candidate-required` plus GoReleaser snapshot |
| Release readiness | local `main` before tag, then tag workflow before publish | prove external state and live smoke evidence | `make release-preflight` exits `0` |
| Release publish | approved semver tag | publish GitHub release artifacts and cask update | GoReleaser release success |
| Post-release smoke | after install | prove operator install path | `lazyss --version`, `lazyss doctor`, one SSH path, one AWS SSM path |

Local stage helpers:

```sh
make check
make fast-pr
make heavy-quality
make release-snapshot
make release-preflight
```

## Branch Protection

Require `ci-required` on `main`. Do not require every component job directly.
The aggregate gate is stable; component jobs can be split or renamed without
needing branch-protection edits.

The aggregate fails when any of these jobs fail:

- `format`
- `mod-tidy`
- `vet`
- `test`
- `script-test`
- `build`
- `smoke-local`
- `lint`
- `govulncheck`

Validate the target state with:

```sh
./scripts/branch-protection-readiness.sh
```

Before requesting owner approval, generate the read-only branch-protection
handoff:

```sh
make branch-protection-plan
```

This writes ignored local files `branch-protection.json` and
`branch-protection.md`. The JSON is the proposed GitHub branch protection API
payload, and the Markdown file includes the exact `gh api --method PUT ...`
command to run only after approval. The generator does not call GitHub APIs or
mutate repository settings.

## Release Candidate Policy

The release-candidate workflow is heavier than fast CI. It should run for:

- pushes to `main` that change release-relevant files, including policy and
  quality-gate files
- pull requests that change release-relevant files, including policy and
  quality-gate files
- pull requests labeled `release-candidate` or `release`
- manual dispatch

The `release-candidate-required` aggregate passes only when the release-candidate
classifier decided proof is required and all release proof jobs passed. If the
classifier decides a PR is not release-relevant, the aggregate passes with an
explicit skip message.

The classifier implementation is covered by `scripts/release_candidate_classify_test.py`
so path-policy changes can be tested before they affect hosted CI.

The GoReleaser snapshot job uploads the generated `dist/` directory as
`goreleaser-snapshot-<sha>` with short retention. Use it to inspect archive
names, checksums, and generated cask output before approving a tag. The
snapshot gate also verifies that the generated private cask uses the expected
download strategy and archive checksums, and that archives contain the expected
installable binaries. On the hosted release-candidate runner, the gate also
extracts the host-matching archive and runs `lazyss --version`.

## Release Policy

Tags are not enough to publish. The release workflow runs readiness before
GoReleaser:

1. Fetch `origin/main`.
2. Verify the tag points at `origin/main`.
3. Write live smoke evidence from `LAZYSS_LIVE_SMOKE_EVIDENCE_JSON` into a
   temporary runner file.
4. Run `./scripts/release-readiness.sh` in tag mode.
5. Upload `release-readiness.json` and `release-readiness.md` as workflow
   artifacts, even when readiness fails after reports are written.
6. Publish with GoReleaser only if readiness exits `0`.

The release workflow still requires owner-managed external state:

- branch protection configured
- Homebrew tap and publishing token approved
- live smoke evidence prepared
- no existing `v0.1.0` release

Do not create tags, branch protection, tap repositories, secrets, or public
release assets without explicit owner approval.

## Evidence Rules

PRs should include exact command output or hosted run URLs for the stage they
claim to satisfy. Release issues should include:

- fast CI run URL
- release-candidate run URL
- local release-readiness output
- hosted `release-readiness-<tag>` artifact for tag runs
- live smoke evidence file path or attached private artifact reference
- final release workflow URL after approval

Never include token values, SSH private keys, AWS credentials, AWS SSO cache
contents, or full environment dumps in PRs, issues, runbooks, generated casks,
or readiness reports.
