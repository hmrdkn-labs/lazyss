#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONFIG="$ROOT/.goreleaser.yaml"
REPO="${LAZYSS_GITHUB_REPO:-hamardikan/lazyss}"
TAP_REPO="${LAZYSS_HOMEBREW_TAP_REPO:-hamardikan/homebrew-tap}"
TAP_SHORT="${LAZYSS_HOMEBREW_TAP_SHORT:-hamardikan/tap}"

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

printf 'LazySS Homebrew readiness audit\n'
printf 'repo: %s\n' "$REPO"
printf 'tap:  %s\n' "$TAP_REPO"
printf 'mode: read-only; this script does not create repos, secrets, tags, or releases\n\n'

cd "$ROOT"

need_command git
need_command gh
need_command brew
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

require_config_text '^homebrew_casks:' "uses GoReleaser homebrew_casks"
require_config_text 'name: lazyss' "cask name is lazyss"
require_config_text 'directory: Casks' "cask output directory is Casks"
require_config_text 'skip_upload: true' "tap publishing remains disabled before approval"
require_config_text 'owner: hamardikan' "tap owner is hamardikan"
require_config_text 'name: homebrew-tap' "tap repository name is homebrew-tap"
require_config_text 'HOMEBREW_TAP_GITHUB_TOKEN' "tap publishing secret is referenced by name only"
require_config_text 'HOMEBREW_GITHUB_API_TOKEN' "private install token env name is documented in generated cask"
require_config_text 'GitHubPrivateRepositoryReleaseDownloadStrategy' "private GitHub download strategy is configured"
require_config_text 'using: GitHubPrivateRepositoryReleaseDownloadStrategy' "cask url uses private download strategy"

if grep -Eiq 'ghp_[A-Za-z0-9_]+|github_pat_[A-Za-z0-9_]+|AWS_SECRET_ACCESS_KEY|AWS_SESSION_TOKEN|BEGIN (OPENSSH|RSA|EC|DSA) PRIVATE KEY' "$CONFIG" docs .github 2>/dev/null; then
	fail "potential credential material found in tracked release docs/config"
else
	ok "no obvious credential material in release docs/config"
fi

printf '\nGitHub state\n'
if gh repo view "$REPO" --json isPrivate,nameWithOwner,defaultBranchRef --jq '"\(.nameWithOwner) private=\(.isPrivate) default=\(.defaultBranchRef.name)"' >/tmp/lazyss-repo-view.$$ 2>/tmp/lazyss-repo-view-err.$$; then
	repo_line="$(cat /tmp/lazyss-repo-view.$$)"
	ok "$repo_line"
	if ! gh repo view "$REPO" --json isPrivate --jq '.isPrivate' | grep -qx 'true'; then
		fail "$REPO is not private"
	fi
else
	warn "could not query $REPO with gh: $(tr '\n' ' ' </tmp/lazyss-repo-view-err.$$)"
fi
rm -f /tmp/lazyss-repo-view.$$ /tmp/lazyss-repo-view-err.$$

if gh repo view "$TAP_REPO" --json isPrivate,nameWithOwner,defaultBranchRef --jq '"\(.nameWithOwner) private=\(.isPrivate) default=\(.defaultBranchRef.name)"' >/tmp/lazyss-tap-view.$$ 2>/tmp/lazyss-tap-view-err.$$; then
	ok "$(cat /tmp/lazyss-tap-view.$$)"
else
	blocker "$TAP_REPO does not exist or is not visible to gh; owner approval is required before creating it"
fi
rm -f /tmp/lazyss-tap-view.$$ /tmp/lazyss-tap-view-err.$$

if gh secret list --repo "$REPO" --json name --jq '.[].name' >/tmp/lazyss-secret-names.$$ 2>/tmp/lazyss-secret-err.$$; then
	if grep -qx 'HOMEBREW_TAP_GITHUB_TOKEN' /tmp/lazyss-secret-names.$$; then
		ok "HOMEBREW_TAP_GITHUB_TOKEN secret name exists"
	else
		blocker "HOMEBREW_TAP_GITHUB_TOKEN secret name is missing; owner approval is required before adding it"
	fi
else
	warn "could not list GitHub secret names: $(tr '\n' ' ' </tmp/lazyss-secret-err.$$)"
fi
rm -f /tmp/lazyss-secret-names.$$ /tmp/lazyss-secret-err.$$

printf '\nHomebrew state\n'
if brew tap | grep -qx "$TAP_SHORT"; then
	ok "$TAP_SHORT is tapped locally"
else
	blocker "$TAP_SHORT is not tapped locally; tap creation/proof still needs owner-approved setup"
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

ok "private Homebrew readiness audit passed"
