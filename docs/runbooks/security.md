# LazySS Security Notes

- LazySS never stores private keys, passwords, AWS tokens, SSO cache contents,
  or environment dumps.
- Commands are executed with explicit argv. LazySS never builds shell strings.
- V1 reads SSH config but does not edit it.
- Local state is written with `0600` file permissions.
- Readiness reports, PRs, runbooks, workflow logs, and generated casks must not
  contain token values, private release asset URLs with embedded credentials,
  AWS credential material, SSH private keys, or SSO cache contents.
- Private Homebrew downloads must resolve `HOMEBREW_GITHUB_API_TOKEN` at runtime
  inside the download strategy and pass authentication through headers. Do not
  return, print, or persist a token-bearing URL.
- Live smoke evidence may contain host labels, region labels, and boolean proof
  fields. It must not contain private keys, AWS access keys, session tokens,
  command environment dumps, or full credential-provider output.
- GitHub workflow secrets are referenced by name only:
  `LAZYSS_LIVE_SMOKE_EVIDENCE_JSON` for release proof and
  `LAZYSS_RELEASE_READINESS_GITHUB_TOKEN` for hosted readiness reads and
  `HOMEBREW_TAP_GITHUB_TOKEN` for approved tap publishing.
