# Security Policy

Report vulnerabilities through a private GitHub security advisory on
`hmrdkn-labs/lazyss` or another agreed private channel with the maintainers.

## Secret Handling

LazySS must never store or log:

- private keys
- passwords
- AWS access keys, session tokens, or SSO cache contents
- GitHub tokens
- full environment dumps

Local state may contain machine names, account IDs, regions, health labels, and
connection history. State files must be written with `0600` permissions.

## Command Safety

LazySS builds commands as executable plus argv. It must not construct shell
strings or rely on shell interpolation for SSH, AWS SSM, Homebrew, or release
automation paths.

## Release Safety

Release automation must not print GitHub token values, AWS credential material,
authorization headers, or operator environment dumps. Homebrew formula
publishing uses a GitHub secret for tap write access, but the generated formula
must contain only public release URLs and checksums.
