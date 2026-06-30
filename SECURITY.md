# Security Policy

LazySS is currently a private project. Report vulnerabilities directly to the
repository owner through the private GitHub repository or an agreed private
channel.

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

Private Homebrew installs require token-backed download. Token values must not
appear in generated casks, docs, CI logs, release notes, or state files.
