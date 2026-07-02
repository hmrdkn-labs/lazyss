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

For a non-interactive adapter smoke against real AWS SSM inventory, run the
opt-in live test. It is excluded from normal CI because it depends on local SSO
state and a real account with at least one managed node:

```sh
LAZYSS_LIVE_AWS_PROFILE=<profile> \
LAZYSS_LIVE_AWS_REGION=<region> \
go test -tags liveaws ./internal/adapters/awsssm -run TestLiveAWSInventoryWithProfile -count=1 -v
```

## Failure Safety

1. Use a bad SSH host and confirm failed connects record failure without
   replacing the last successful connection.
2. Use expired or missing AWS credentials and confirm SSH inventory still works.
3. Inspect state/log output and confirm no private keys, tokens, SSO cache data,
   or environment dumps are written.
4. Confirm local state file permissions are `0600`.

## Release Evidence File

Before `v0.1.0`, record the live SSH and AWS SSM smoke result in a local JSON
file that is not committed:

```sh
make live-smoke-evidence-template
```

The helper writes `live-smoke-evidence.json` with file mode `0600`, the current
commit, the target release version, and deliberately failing boolean fields.
Edit it only after the real smoke checks pass. Fill in only non-secret labels:

- `target_version`: release version, for example `v0.1.0`
- `commit`: full commit SHA that was smoked
- `checked_at`: ISO-8601 timestamp
- `ssh.host_label`: SSH config alias or redacted host label
- `aws_ssm.profile_label`: non-secret profile label
- `aws_ssm.region`: AWS region
- `aws_ssm.target_label`: managed-node ID or redacted target label

Do not include host passwords, private keys, AWS credentials, SSO cache data,
GitHub tokens, environment dumps, release asset URLs with credentials, or full terminal
logs.

Validate the evidence directly:

```sh
python3 scripts/live_smoke_evidence.py validate \
  --file live-smoke-evidence.json \
  --target-version v0.1.0 \
  --commit "$(git rev-parse HEAD)"
```

Then validate it with the full release readiness audit:

```sh
LAZYSS_LIVE_SMOKE_EVIDENCE=live-smoke-evidence.json ./scripts/release-readiness.sh
```

The readiness audit checks that the evidence matches the current commit and
target release version, includes passed SSH and AWS SSM smoke fields, records
the required safety checks, and does not contain obvious credential material.
