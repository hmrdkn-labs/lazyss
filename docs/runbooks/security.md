# LazySS Security Notes

- LazySS never stores private keys, passwords, AWS tokens, SSO cache contents,
  or environment dumps.
- Commands are executed with explicit argv. LazySS never builds shell strings.
- V1 reads SSH config but does not edit it.
- Local state is written with `0600` file permissions.
