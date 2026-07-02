# AWS Profile SSO TUI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make LazySS discover, persist, select, and refresh AWS profiles from the TUI so SSO-backed AWS SSM inventory works without requiring the operator to remember command-line flags.

**Architecture:** Keep AWS credentials outside LazySS. Persist only safe operator preferences in the existing `statejson` store. Add a small AWS CLI adapter for profile-name discovery and SSO login, then let the TUI switch profile by rebuilding the app inventory service through a runtime callback.

**Tech Stack:** Go, Bubble Tea v2, AWS SDK v2, AWS CLI, existing JSON state store.

---

### Task 1: Persist Safe AWS Preferences

**Files:**
- Modify: `internal/domain/domain.go`
- Modify: `internal/ports/ports.go`
- Modify: `internal/adapters/statejson/state.go`
- Test: `internal/adapters/statejson/state_test.go`

- [ ] Add `domain.OperatorPreferences` with `AWSProfile` and `AWSRegion`.
- [ ] Add `LoadPreferences` and `SavePreferences` to a new `ports.PreferenceStore` interface.
- [ ] Store preferences under top-level `preferences` in `state.json`.
- [ ] Verify JSON writes remain atomic and mode `0600`.

### Task 2: Add AWS Profile CLI Adapter

**Files:**
- Create: `internal/adapters/awsconfig/awsconfig.go`
- Test: `internal/adapters/awsconfig/awsconfig_test.go`

- [ ] Implement `ListProfiles(ctx)` by running `aws configure list-profiles`.
- [ ] Implement `Login(ctx, profile)` by running `aws sso login --profile <profile>`.
- [ ] Parse profile names from stdout only; do not read credential files or print tokens.
- [ ] Reject blank profile login.

### Task 3: Wire Startup Profile Resolution

**Files:**
- Modify: `cmd/lazyss/main.go`
- Test: `cmd/lazyss/main_test.go`

- [ ] Resolve effective AWS profile as CLI flag first, then persisted preference, then empty SDK default.
- [ ] Keep `--aws-profile` as an override that does not require persisted state.
- [ ] Pass profile metadata and rebuild callback into TUI runtime.

### Task 4: Add TUI Profile Selection And Login

**Files:**
- Modify: `internal/tui/model.go`
- Test: `internal/tui/model_test.go`

- [ ] Add `P` to open an AWS profile picker.
- [ ] Add `L` to run AWS SSO login for the selected profile.
- [ ] Save selected profile to preferences and refresh inventory.
- [ ] Show profile-aware auth hints without exposing raw AWS SDK errors.

### Task 5: Verify Local Integration

**Files:**
- Modify as needed only if tests reveal gaps.

- [ ] Run focused Go tests for state, AWS config, command wiring, and TUI.
- [ ] Run full gates: `gofmt -l .`, `go vet ./...`, `go test -race -coverprofile=coverage.out ./...`, coverage summary, and build.
- [ ] Build `./bin/lazyss`.
- [ ] Smoke `./bin/lazyss --aws-profile hmrdkn-dev1` and confirm AWS inventory appears.
- [ ] Smoke plain `./bin/lazyss` after selecting/persisting the profile and confirm AWS inventory appears or the profile/login hint is clear.
