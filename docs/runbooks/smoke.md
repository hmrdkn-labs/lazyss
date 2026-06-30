# LazySS Smoke Runbook

## Direct SSH

1. Add or use an existing host in `~/.ssh/config`.
2. Run `lazyss --source ssh`.
3. Select the host, press `g`, then press `Enter`.

## AWS SSM

1. Ensure AWS CLI and `session-manager-plugin` are installed.
2. Run `lazyss doctor --aws-profile <profile> --aws-region <region>`.
3. Run `lazyss --source aws --aws-profile <profile> --aws-region <region>`.
4. Select an SSM-ready machine, press `g`, then press `Enter`.

## Failure Safety

1. Use a bad SSH host and confirm failed connects record failure without
   replacing the last successful connection.
2. Use expired or missing AWS credentials and confirm SSH inventory still works.
3. Inspect state/log output and confirm no private keys, tokens, SSO cache data,
   or environment dumps are written.
