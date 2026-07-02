# LazySS Security Notes

- LazySS never stores private keys, passwords, AWS tokens, SSO cache contents,
  or environment dumps.
- Commands are executed with explicit argv. LazySS never builds shell strings.
- V1 reads SSH config but does not edit it.
- Local state is written with `0600` file permissions.
- Readiness reports, PRs, runbooks, workflow logs, and generated tap files must
  not contain token values, release asset URLs with embedded credentials, AWS
  credential material, SSH private keys, or SSO cache contents.
- Public Homebrew formulae must use public release URLs and checksums only.
  They must not contain token-backed download strategies or authorization
  header templates.
- Live smoke evidence may contain host labels, region labels, and boolean proof
  fields. It must not contain private keys, AWS access keys, session tokens,
  command environment dumps, or full credential-provider output.
- Release artifact checks verify that the six expected platform archives exist,
  contain non-empty binaries, preserve executable mode for tar archives, match
  `checksums.txt`, and generate a Homebrew formula with public release URLs and
  matching checksums before a tag is approved or installed. The
  release-candidate workflow also executes the host-matching archived binary
  with `--version`.
- Hosted release readiness reports are uploaded as workflow artifacts for audit,
  but they must contain only check levels and messages. They must not include
  secrets or credential-derived output.
- GitHub workflow secrets are referenced by name only:
  `LAZYSS_LIVE_SMOKE_EVIDENCE_JSON` for release proof and
  `LAZYSS_RELEASE_READINESS_GITHUB_TOKEN` for hosted readiness reads and
  `HOMEBREW_TAP_GITHUB_TOKEN` for approved tap publishing.
