# Lazy Secure Shell

`lazyss` is a terminal machine cockpit for direct SSH and AWS Systems Manager
Session Manager. It discovers machines from read-only SSH config and AWS
SSM/EC2 inventory, shows method-specific reachability, launches secure
sessions, and stores local operator memory such as pins, notes, checks, and
connection history.

## Scope

V1 supports:

- read-only `~/.ssh/config` inventory
- AWS SSM + EC2 inventory
- direct `ssh` sessions
- `aws ssm start-session` sessions
- manual health checks
- local JSON state under the OS user config directory

V1 deliberately does not edit SSH config, write ProxyCommand entries, run a
monitoring daemon, store secrets, or implement non-AWS cloud adapters.

## Usage

```sh
lazyss
lazyss doctor
lazyss --source all
lazyss --source ssh --ssh-config ~/.ssh/config
lazyss --source aws --aws-profile prod --aws-region ap-southeast-1
```

## Verification

```sh
gofmt -l .
go vet ./...
go test -race ./...
go build ./cmd/lazyss
```
