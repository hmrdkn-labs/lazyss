#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONFIG="$ROOT/.goreleaser.yaml"
RELEASE_WORKFLOW="$ROOT/.github/workflows/release.yml"
FORMULA_GENERATOR="$ROOT/scripts/homebrew_formula.py"
REPO="${LAZYSS_GITHUB_REPO:-hmrdkn-labs/lazyss}"
TAP_REPO="${LAZYSS_HOMEBREW_TAP_REPO:-hmrdkn-labs/homebrew-tap}"
TAP_SHORT="${LAZYSS_HOMEBREW_TAP_SHORT:-hmrdkn-labs/tap}"
READINESS_MODE="${LAZYSS_HOMEBREW_READINESS_MODE:-local}"
REQUIRE_TAP_UPLOAD="${LAZYSS_REQUIRE_HOMEBREW_TAP_UPLOAD:-0}"

failures=0
blockers=0

ok() {
	printf '[ok] %s\n' "$*"
}

warn() {
	printf '[warn] %s\n' "$*"
}

fail() {
	failures=$((failures + 1))
	printf '[fail] %s\n' "$*" >&2
}

blocker() {
	blockers=$((blockers + 1))
	printf '[blocker] %s\n' "$*" >&2
}

need_command() {
	local name="$1"
	if command -v "$name" >/dev/null 2>&1; then
		ok "$name available: $(command -v "$name")"
	else
		fail "$name is not available"
	fi
}

require_config_text() {
	local pattern="$1"
	local label="$2"
	if grep -q "$pattern" "$CONFIG"; then
		ok "$label"
	else
		fail "$label missing in .goreleaser.yaml"
	fi
}

forbid_config_text() {
	local pattern="$1"
	local label="$2"
	if grep -q "$pattern" "$CONFIG"; then
		fail "$label must not appear in public Homebrew config"
	else
		ok "$label absent"
	fi
}

require_file() {
	local path="$1"
	local label="$2"
	if [ -f "$path" ]; then
		ok "$label"
	else
		fail "$label missing"
	fi
}

require_workflow_text() {
	local pattern="$1"
	local label="$2"
	if grep -q "$pattern" "$RELEASE_WORKFLOW"; then
		ok "$label"
	else
		fail "$label missing in release workflow"
	fi
}

printf 'LazySS Homebrew readiness audit\n'
printf 'repo: %s\n' "$REPO"
printf 'tap:  %s\n' "$TAP_REPO"
printf 'mode: %s\n' "$READINESS_MODE"
printf 'mutation: read-only; this script does not create repos, secrets, tags, or releases\n\n'

case "$READINESS_MODE" in
	local | hosted)
		;;
	*)
		fail "LAZYSS_HOMEBREW_READINESS_MODE must be local or hosted, got: $READINESS_MODE"
		;;
esac

case "$REQUIRE_TAP_UPLOAD" in
	0 | 1)
		;;
	*)
		fail "LAZYSS_REQUIRE_HOMEBREW_TAP_UPLOAD must be 0 or 1, got: $REQUIRE_TAP_UPLOAD"
		;;
esac

if [ "$failures" -gt 0 ]; then
	printf '[fail] %s local readiness failure(s)\n' "$failures" >&2
	exit 1
fi

cd "$ROOT"

need_command git
need_command gh
if [ "$READINESS_MODE" = "local" ]; then
	need_command brew
else
	warn "local brew tap checks skipped in hosted readiness mode"
fi
if command -v goreleaser >/dev/null 2>&1; then
	ok "goreleaser available: $(command -v goreleaser)"
else
	warn "goreleaser is not installed locally; hosted CI still runs GoReleaser snapshot"
fi

printf '\nConfiguration\n'
if [ -f "$CONFIG" ]; then
	ok ".goreleaser.yaml exists"
else
	fail ".goreleaser.yaml missing"
fi

require_file "$FORMULA_GENERATOR" "Homebrew formula generator exists"
require_file "$RELEASE_WORKFLOW" "release workflow exists"
require_workflow_text 'scripts/homebrew_formula.py generate' "release workflow generates formula"
require_workflow_text 'repository: hmrdkn-labs/homebrew-tap' "release workflow checks out public tap"
require_workflow_text 'HOMEBREW_TAP_GITHUB_TOKEN' "release workflow references tap publishing secret by name only"
require_workflow_text 'Formula/lazyss.rb' "release workflow writes Formula/lazyss.rb"
forbid_config_text '^brews:' "deprecated GoReleaser brews"
forbid_config_text '^homebrew_casks:' "homebrew_casks"
forbid_config_text 'directory: Casks' "Casks output directory"
forbid_config_text 'HOMEBREW_GITHUB_API_TOKEN' "private release download token"
forbid_config_text 'GitHubPrivateRepositoryReleaseDownloadStrategy' "private GitHub download strategy"
forbid_config_text 'Authorization: Bearer' "authorization header template"
if grep -q 'skip_upload: true' "$CONFIG"; then
	if [ "$REQUIRE_TAP_UPLOAD" = "1" ]; then
		fail "tap publishing is required but .goreleaser.yaml still has skip_upload: true"
	else
		warn "tap publishing remains disabled with skip_upload: true"
	fi
else
	ok "tap publishing is enabled for the approved tap"
fi

if grep -Eiq 'ghp_[A-Za-z0-9_]+|github_pat_[A-Za-z0-9_]+|AWS_SECRET_ACCESS_KEY|AWS_SESSION_TOKEN|BEGIN (OPENSSH|RSA|EC|DSA) PRIVATE KEY' "$CONFIG" docs .github 2>/dev/null; then
	fail "potential credential material found in tracked release docs/config"
else
	ok "no obvious credential material in release docs/config"
fi

printf '\nGitHub state\n'
repo_out="$(mktemp "${TMPDIR:-/tmp}/lazyss-repo-view.XXXXXX")"
repo_err="$(mktemp "${TMPDIR:-/tmp}/lazyss-repo-view-err.XXXXXX")"
if gh repo view "$REPO" --json isPrivate,nameWithOwner,defaultBranchRef --jq '"\(.nameWithOwner) private=\(.isPrivate) default=\(.defaultBranchRef.name)"' >"$repo_out" 2>"$repo_err"; then
	repo_line="$(cat "$repo_out")"
	ok "$repo_line"
	if gh repo view "$REPO" --json isPrivate --jq '.isPrivate' | grep -qx 'false'; then
		ok "$REPO is public"
	else
		blocker "$REPO is still private; public release requires repository visibility change"
	fi
else
	warn "could not query $REPO with gh: $(tr '\n' ' ' <"$repo_err")"
fi
rm -f "$repo_out" "$repo_err"

tap_out="$(mktemp "${TMPDIR:-/tmp}/lazyss-tap-view.XXXXXX")"
tap_err="$(mktemp "${TMPDIR:-/tmp}/lazyss-tap-view-err.XXXXXX")"
if gh repo view "$TAP_REPO" --json isPrivate,nameWithOwner,defaultBranchRef --jq '"\(.nameWithOwner) private=\(.isPrivate) default=\(.defaultBranchRef.name)"' >"$tap_out" 2>"$tap_err"; then
	tap_line="$(cat "$tap_out")"
	ok "$tap_line"
	if gh repo view "$TAP_REPO" --json isPrivate --jq '.isPrivate' | grep -qx 'false'; then
		ok "$TAP_REPO is public"
	else
		blocker "$TAP_REPO is private; public Homebrew tap should be public"
	fi
else
	blocker "$TAP_REPO does not exist or is not visible to gh; approval is required before creating it"
fi
rm -f "$tap_out" "$tap_err"

secret_names="$(mktemp "${TMPDIR:-/tmp}/lazyss-secret-names.XXXXXX")"
secret_err="$(mktemp "${TMPDIR:-/tmp}/lazyss-secret-err.XXXXXX")"
if gh secret list --repo "$REPO" --json name --jq '.[].name' >"$secret_names" 2>"$secret_err"; then
	if grep -qx 'HOMEBREW_TAP_GITHUB_TOKEN' "$secret_names"; then
		ok "HOMEBREW_TAP_GITHUB_TOKEN secret name exists"
	else
		blocker "HOMEBREW_TAP_GITHUB_TOKEN secret name is missing; it is required for release publishing"
	fi
else
	if [ "$REQUIRE_TAP_UPLOAD" = "1" ]; then
		blocker "could not list GitHub secret names while tap upload is required: $(tr '\n' ' ' <"$secret_err")"
	else
		warn "could not list GitHub secret names: $(tr '\n' ' ' <"$secret_err")"
	fi
fi
rm -f "$secret_names" "$secret_err"

printf '\nHomebrew state\n'
if [ "$READINESS_MODE" = "hosted" ]; then
	warn "$TAP_SHORT local tap state skipped in hosted readiness mode"
elif brew tap | grep -qx "$TAP_SHORT"; then
	ok "$TAP_SHORT is tapped locally"
else
	blocker "$TAP_SHORT is not tapped locally; run brew tap $TAP_SHORT after tap creation"
fi

printf '\nSummary\n'
if [ "$failures" -gt 0 ]; then
	printf '[fail] %s local readiness failure(s)\n' "$failures" >&2
	exit 1
fi
if [ "$blockers" -gt 0 ]; then
	printf '[blocker] %s approval/external-state blocker(s)\n' "$blockers" >&2
	exit 2
fi

ok "public Homebrew readiness audit passed"
