# ADR 0004: CLI-Backed Cloud Providers

Status: Accepted

Date: 2026-11-28

## Context

LazySS v0.1 and v0.2 discover machines from SSH config and AWS SSM/EC2 only.
v0.3 adds Google Cloud Compute Engine and Azure virtual machines. Both clouds
already ship command-line tools, `gcloud` and `az`, that hold the operator's
login and print JSON. Each cloud also has a Go SDK, which would bring its own
auth flow, token cache, and dependency tree into a single-binary terminal tool.

## Decision

- The Google Cloud and Azure adapters run `gcloud` and `az` as child processes.
  Every call is an argv slice with JSON output (`--format=json`, `-o json`).
  No shell strings.
- LazySS adds no Google Cloud or Azure SDK module.
- LazySS reuses the operator's existing CLI login. It never reads, stores,
  copies, or logs tokens, refresh tokens, credential files, or CLI token caches.
  Stored state keeps identifiers only: the GCP project ID and Azure
  subscription ID the operator picked.
- Login is delegated. `L` hands the terminal to `gcloud auth login` or `az login`.
- Inventory, connection, and health use the ports from ADR 0001. Health stays a
  local, method-specific observation. It never claims IAP, Entra ID, RBAC, or
  Bastion SKU behavior that the adapters cannot observe.
- The adapters live in `internal/adapters/gcloud` and `internal/adapters/azcli`.
- Values that could be read as CLI flags (leading `-`) are rejected before any
  command runs.

## Consequences

- Installing and logging in to the CLIs is the operator's job. `lazyss doctor`
  reports a missing or logged-out CLI; it does not install or log in.
- CLI output-format drift is a risk. The parsers read specific JSON fields from
  `gcloud compute instances list`, `gcloud projects list`, `gcloud auth list`,
  `az vm list -d`, `az network nic list`, `az network bastion list`,
  `az account show`, `az account list`, and `az extension list`. An unreadable
  reply degrades that provider's status instead of crashing the cockpit. A
  renamed field is not an error: it decodes as an empty value. Fixture tests
  must be kept in step with the CLI output.
- Test strategy: unit tests inject a fake runner that returns JSON from
  `testdata/` fixtures, and a fake dialer for TCP probes. The GCP and Azure
  packages have no live-cloud test. Real GCP and Azure smoke is manual, optional
  evidence (see the smoke runbook).
- Bastion matching covers only a Bastion in the same VNet as one of the VM's
  NICs. Bastions reached through VNet peering are not detected, so those VMs get
  no `az-bastion-ssh` method.
- `az ssh vm` needs the `ssh` CLI extension; `lazyss doctor` reports its absence.
- IAP reachability depends on firewall rules and IAM that LazySS cannot see.
  `gcloud-iap-ssh` health reads inventory only.
- An SDK-based adapter would need a new ADR that supersedes this one.
