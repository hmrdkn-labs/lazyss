# LazySS Smoke Runbook

Run smoke tests with a built binary:

```sh
make build
./bin/lazyss --version
```

## Automated Local Smoke

Run the safe local smoke gate before opening a release PR:

```sh
make smoke-local
```

This gate requires `expect` because it starts the TUI in a pseudo-terminal. CI
installs `expect` on the Ubuntu runner before running the hosted smoke step. The
gate does not use real SSH or AWS targets. It verifies:

- `make build` produces a runnable binary
- `lazyss --version` returns a LazySS version string
- `lazyss doctor` runs with EC2 metadata disabled and does not print credential
  material
- the TUI renders a temporary SSH inventory row
- the temporary SSH config is not mutated

`lazyss doctor` may exit non-zero during this smoke when AWS credentials,
`session-manager-plugin`, or other local prerequisites are missing. That is
acceptable for the local smoke as long as the command returns structured doctor
output and does not leak credentials.

## Release Candidate Direct SSH

1. Add or use an existing host in `~/.ssh/config`.
2. Run `./bin/lazyss --source ssh`.
3. Select the host, press `g`, then press `Enter`.
4. Confirm `~/.ssh/config` was not modified.

## Release Candidate AWS SSM

1. Ensure AWS CLI and `session-manager-plugin` are installed.
2. Run `./bin/lazyss doctor --aws-profile <profile> --aws-region <region>`.
3. Run `./bin/lazyss --source aws --aws-profile <profile> --aws-region <region>`.
4. Select an SSM-ready machine, press `g`, then press `Enter`.

## Failure Safety

1. Use a bad SSH host and confirm failed connects record failure without
   replacing the last successful connection.
2. Use expired or missing AWS credentials and confirm SSH inventory still works.
3. Inspect state/log output and confirm no private keys, tokens, SSO cache data,
   or environment dumps are written.
4. Confirm local state file permissions are `0600`.
