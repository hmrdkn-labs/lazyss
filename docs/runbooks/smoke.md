# LazySS Smoke Runbook

Run smoke tests with a built binary:

```sh
make build
./bin/lazyss --version
```

## Direct SSH

1. Add or use an existing host in `~/.ssh/config`.
2. Run `./bin/lazyss --source ssh`.
3. Select the host, press `g`, then press `Enter`.
4. Confirm `~/.ssh/config` was not modified.

## AWS SSM

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
