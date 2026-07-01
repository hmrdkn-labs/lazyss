#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPO="${LAZYSS_GITHUB_REPO:-hamardikan/lazyss}"
RELEASE_VERSION="${LAZYSS_RELEASE_VERSION:-v0.1.0}"
RELEASE_CANDIDATE_WORKFLOW="${LAZYSS_RELEASE_CANDIDATE_WORKFLOW:-Release Candidate}"
FAST_CI_WORKFLOW="${LAZYSS_FAST_CI_WORKFLOW:-CI}"
READINESS_MODE="${LAZYSS_RELEASE_READINESS_MODE:-main}"
SKIP_OPERATOR_RUNTIME_TOOLS="${LAZYSS_SKIP_OPERATOR_RUNTIME_TOOL_CHECKS:-0}"
REPORT_JSON="${LAZYSS_RELEASE_READINESS_JSON:-}"
REPORT_MARKDOWN="${LAZYSS_RELEASE_READINESS_MARKDOWN:-}"
LIVE_SMOKE_EVIDENCE="${LAZYSS_LIVE_SMOKE_EVIDENCE:-}"
HOMEBREW_PRIVATE_EVIDENCE="${LAZYSS_HOMEBREW_PRIVATE_EVIDENCE:-}"
REQUIRE_HOMEBREW_PRIVATE_EVIDENCE="${LAZYSS_REQUIRE_HOMEBREW_PRIVATE_EVIDENCE:-0}"

failures=0
blockers=0
warnings=0
branch=""
head=""
short_head=""
checks_file="$(mktemp "${TMPDIR:-/tmp}/lazyss-readiness-checks.XXXXXX")"

cleanup() {
	rm -f "$checks_file"
}
trap cleanup EXIT

record_check() {
	local level="$1"
	shift
	printf '%s\t%s\n' "$level" "$*" >>"$checks_file"
}

ok() {
	record_check ok "$*"
	printf '[ok] %s\n' "$*"
}

warn() {
	warnings=$((warnings + 1))
	record_check warn "$*"
	printf '[warn] %s\n' "$*"
}

fail() {
	failures=$((failures + 1))
	record_check fail "$*"
	printf '[fail] %s\n' "$*"
}

blocker() {
	blockers=$((blockers + 1))
	record_check blocker "$*"
	printf '[blocker] %s\n' "$*"
}

write_reports() {
	local status="$1"
	if [ -z "$REPORT_JSON" ] && [ -z "$REPORT_MARKDOWN" ]; then
		return
	fi
	if ! command -v python3 >/dev/null 2>&1; then
		printf '[warn] python3 unavailable; skipping structured readiness reports\n' >&2
		return
	fi

	python3 - "$checks_file" "$REPORT_JSON" "$REPORT_MARKDOWN" "$status" "$REPO" "$RELEASE_VERSION" "$branch" "$head" "$failures" "$blockers" "$warnings" <<'PY'
import datetime
import json
import pathlib
import sys

checks_path, json_path, markdown_path, status, repo, release_version, branch, head, failures, blockers, warnings = sys.argv[1:]
checks = []
with open(checks_path, encoding="utf-8") as handle:
    for raw in handle:
        raw = raw.rstrip("\n")
        if not raw:
            continue
        level, message = raw.split("\t", 1)
        checks.append({"level": level, "message": message})

payload = {
    "repo": repo,
    "target_version": release_version,
    "status": status,
    "branch": branch,
    "head": head,
    "failures": int(failures),
    "blockers": int(blockers),
    "warnings": int(warnings),
    "generated_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
    "checks": checks,
}

if json_path:
    pathlib.Path(json_path).write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")

if markdown_path:
    def escape_cell(value: str) -> str:
        return value.replace("|", "\\|").replace("\n", " ")

    lines = [
        "# LazySS Release Readiness",
        "",
        f"- Repo: `{repo}`",
        f"- Target version: `{release_version}`",
        f"- Status: `{status}`",
        f"- Branch: `{branch or 'unknown'}`",
        f"- Head: `{head or 'unknown'}`",
        f"- Failures: `{failures}`",
        f"- Blockers: `{blockers}`",
        f"- Warnings: `{warnings}`",
        "",
        "| Level | Check |",
        "| --- | --- |",
    ]
    lines.extend(f"| {escape_cell(item['level'])} | {escape_cell(item['message'])} |" for item in checks)
    pathlib.Path(markdown_path).write_text("\n".join(lines) + "\n", encoding="utf-8")
PY
}

finish() {
	local exit_code="$1"
	local status="$2"
	write_reports "$status"
	exit "$exit_code"
}

need_command() {
	local name="$1"
	if command -v "$name" >/dev/null 2>&1; then
		ok "$name available: $(command -v "$name")"
	else
		fail "$name is not available"
	fi
}

latest_run_json() {
	local workflow="$1"
	gh run list \
		--repo "$REPO" \
		--workflow "$workflow" \
		--branch main \
		--limit 1 \
		--json databaseId,headSha,status,conclusion,url \
		--jq '.[0] // empty'
}

check_latest_workflow() {
	local workflow="$1"
	local head="$2"
	local json message
	if ! json="$(latest_run_json "$workflow" 2>/tmp/lazyss-run-err.$$)" || [ -z "$json" ]; then
		message="could not query latest $workflow run: $(tr '\n' ' ' </tmp/lazyss-run-err.$$)"
		if [ "$READINESS_MODE" = "tag" ]; then
			blocker "$message"
		else
			warn "$message"
		fi
		rm -f /tmp/lazyss-run-err.$$
		return
	fi
	rm -f /tmp/lazyss-run-err.$$

	local run_sha status conclusion url
	run_sha="$(printf '%s\n' "$json" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("headSha",""))')"
	status="$(printf '%s\n' "$json" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("status",""))')"
	conclusion="$(printf '%s\n' "$json" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("conclusion",""))')"
	url="$(printf '%s\n' "$json" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("url",""))')"

	if [ "$run_sha" != "$head" ]; then
		blocker "$workflow latest run is for $run_sha, not current main $head"
		return
	fi
	if [ "$status" = "completed" ] && [ "$conclusion" = "success" ]; then
		ok "$workflow passed for $head ($url)"
	else
		blocker "$workflow is not green for $head: status=$status conclusion=$conclusion ($url)"
	fi
}

check_live_smoke_evidence() {
	if [ -z "$LIVE_SMOKE_EVIDENCE" ]; then
		if [ "${LAZYSS_LIVE_SSH_SMOKE_PASSED:-}" = "1" ] || [ "${LAZYSS_LIVE_AWS_SSM_SMOKE_PASSED:-}" = "1" ]; then
			warn "legacy live smoke env flags are ignored; set LAZYSS_LIVE_SMOKE_EVIDENCE to a validated evidence file"
		fi
		blocker "live smoke evidence file is not provided; set LAZYSS_LIVE_SMOKE_EVIDENCE after real SSH/AWS SSM smokes"
		return
	fi

	if [ ! -f "$LIVE_SMOKE_EVIDENCE" ]; then
		blocker "live smoke evidence file does not exist: $LIVE_SMOKE_EVIDENCE"
		return
	fi

	local output rc
	set +e
	output="$(python3 "$ROOT/scripts/live_smoke_evidence.py" validate --file "$LIVE_SMOKE_EVIDENCE" --target-version "$RELEASE_VERSION" --commit "$head")"
	rc=$?
	set -e

	while IFS=$'\t' read -r level message; do
		[ -n "$level" ] || continue
		case "$level" in
			ok) ok "$message" ;;
			warn) warn "$message" ;;
			blocker) blocker "$message" ;;
			fail) fail "$message" ;;
			*) fail "live smoke evidence validator returned unknown level: $level" ;;
		esac
	done <<EOF
$output
EOF

	if [ "$rc" -ne 0 ] && [ "$rc" -ne 1 ] && [ "$rc" -ne 2 ]; then
		fail "live smoke evidence validator failed with exit $rc"
	fi
}

check_homebrew_private_evidence() {
	if [ -z "$HOMEBREW_PRIVATE_EVIDENCE" ]; then
		if [ "$REQUIRE_HOMEBREW_PRIVATE_EVIDENCE" = "1" ]; then
			blocker "private Homebrew install evidence file is not provided; set LAZYSS_HOMEBREW_PRIVATE_EVIDENCE after token-backed brew install smoke"
		else
			warn "private Homebrew install evidence is not required for pre-publish readiness; capture post-publish evidence after the first token-backed cask install"
		fi
		return
	fi

	if [ ! -f "$HOMEBREW_PRIVATE_EVIDENCE" ]; then
		blocker "private Homebrew install evidence file does not exist: $HOMEBREW_PRIVATE_EVIDENCE"
		return
	fi

	local output rc
	set +e
	output="$(python3 "$ROOT/scripts/homebrew_private_evidence.py" validate --file "$HOMEBREW_PRIVATE_EVIDENCE" --target-version "$RELEASE_VERSION" --commit "$head")"
	rc=$?
	set -e

	while IFS=$'\t' read -r level message; do
		[ -n "$level" ] || continue
		case "$level" in
			ok) ok "$message" ;;
			warn) warn "$message" ;;
			blocker) blocker "$message" ;;
			fail) fail "$message" ;;
			*) fail "private Homebrew evidence validator returned unknown level: $level" ;;
		esac
	done <<EOF
$output
EOF

	if [ "$rc" -ne 0 ] && [ "$rc" -ne 1 ] && [ "$rc" -ne 2 ]; then
		fail "private Homebrew evidence validator failed with exit $rc"
	fi
}

printf 'LazySS release readiness audit\n'
printf 'repo: %s\n' "$REPO"
printf 'target version: %s\n' "$RELEASE_VERSION"
printf 'mode: %s\n' "$READINESS_MODE"
printf 'mutation: read-only; this script does not create repos, secrets, branch protection, tags, or releases\n\n'

case "$READINESS_MODE" in
	main | tag)
		;;
	*)
		fail "LAZYSS_RELEASE_READINESS_MODE must be main or tag, got: $READINESS_MODE"
		;;
esac

case "$REQUIRE_HOMEBREW_PRIVATE_EVIDENCE" in
	0 | 1)
		;;
	*)
		fail "LAZYSS_REQUIRE_HOMEBREW_PRIVATE_EVIDENCE must be 0 or 1, got: $REQUIRE_HOMEBREW_PRIVATE_EVIDENCE"
		;;
esac

if [ "$failures" -gt 0 ]; then
	printf '[fail] %s release readiness failure(s)\n' "$failures" >&2
	finish 1 failed
fi

cd "$ROOT"

printf 'Tools\n'
need_command git
need_command gh
need_command python3
if [ "$SKIP_OPERATOR_RUNTIME_TOOLS" = "1" ]; then
	warn "operator runtime tool checks skipped; live smoke evidence remains required"
else
	need_command ssh
	need_command aws
	if command -v session-manager-plugin >/dev/null 2>&1; then
		ok "session-manager-plugin available: $(command -v session-manager-plugin)"
	else
		blocker "session-manager-plugin is missing; AWS SSM live smoke cannot pass"
	fi
fi

printf '\nGit workspace\n'
branch="$(git branch --show-current)"
head="$(git rev-parse HEAD)"
short_head="$(git rev-parse --short HEAD)"
if [ "$READINESS_MODE" = "tag" ]; then
	if git rev-parse -q --verify "refs/tags/$RELEASE_VERSION" >/tmp/lazyss-release-tag.$$.out; then
		tag_head="$(git rev-list -n 1 "$RELEASE_VERSION")"
		if [ "$tag_head" = "$head" ]; then
			ok "release tag $RELEASE_VERSION points at current checkout $short_head"
		else
			blocker "release tag $RELEASE_VERSION points at $tag_head, not current checkout $head"
		fi
	else
		blocker "release tag $RELEASE_VERSION is not available locally"
	fi
	rm -f /tmp/lazyss-release-tag.$$.out

	if git rev-parse --verify origin/main >/dev/null 2>&1; then
		origin_head="$(git rev-parse origin/main)"
		if [ "$origin_head" = "$head" ]; then
			ok "release tag commit matches origin/main"
		else
			blocker "release tag commit does not match origin/main"
		fi
	else
		blocker "origin/main is not available locally for release tag verification"
	fi
else
	if [ "$branch" = "main" ]; then
		ok "on main at $short_head"
	else
		blocker "not on main: $branch"
	fi
fi

if [ -z "$(git status --porcelain)" ]; then
	ok "working tree is clean"
else
	blocker "working tree has uncommitted changes"
fi

if [ "$READINESS_MODE" = "main" ] && git rev-parse --verify origin/main >/dev/null 2>&1; then
	origin_head="$(git rev-parse origin/main)"
	if [ "$origin_head" = "$head" ]; then
		ok "local main matches origin/main"
	else
		blocker "local HEAD does not match origin/main"
	fi
elif [ "$READINESS_MODE" = "main" ]; then
	warn "origin/main is not available locally"
fi

printf '\nGitHub state\n'
if gh repo view "$REPO" --json isPrivate,nameWithOwner,defaultBranchRef --jq '"\(.nameWithOwner) private=\(.isPrivate) default=\(.defaultBranchRef.name)"' >/tmp/lazyss-repo.$$.out 2>/tmp/lazyss-repo.$$.err; then
	repo_line="$(cat /tmp/lazyss-repo.$$.out)"
	ok "$repo_line"
	if ! gh repo view "$REPO" --json isPrivate --jq '.isPrivate' | grep -qx 'true'; then
		fail "$REPO is not private"
	fi
else
	warn "could not query $REPO: $(tr '\n' ' ' </tmp/lazyss-repo.$$.err)"
fi
rm -f /tmp/lazyss-repo.$$.out /tmp/lazyss-repo.$$.err

set +e
"$ROOT/scripts/branch-protection-readiness.sh" >/tmp/lazyss-branch-protection.$$.out 2>&1
branch_protection_rc=$?
set -e
case "$branch_protection_rc" in
	0)
		ok "branch protection readiness passed"
		;;
	2)
		blocker "branch protection readiness has approval/external-state blockers"
		sed 's/^/  /' /tmp/lazyss-branch-protection.$$.out
		;;
	*)
		fail "branch protection readiness failed with exit $branch_protection_rc"
		sed 's/^/  /' /tmp/lazyss-branch-protection.$$.out
		;;
esac
rm -f /tmp/lazyss-branch-protection.$$.out

if gh pr list --repo "$REPO" --state open --json number --jq 'length' >/tmp/lazyss-open-prs.$$ 2>/tmp/lazyss-open-prs-err.$$; then
	open_prs="$(cat /tmp/lazyss-open-prs.$$)"
	if [ "$open_prs" = "0" ]; then
		ok "no open pull requests"
	else
		blocker "$open_prs open pull request(s) remain"
	fi
else
	warn "could not list open PRs: $(tr '\n' ' ' </tmp/lazyss-open-prs-err.$$)"
fi
rm -f /tmp/lazyss-open-prs.$$ /tmp/lazyss-open-prs-err.$$

check_latest_workflow "$FAST_CI_WORKFLOW" "$head"
check_latest_workflow "$RELEASE_CANDIDATE_WORKFLOW" "$head"

printf '\nRelease state\n'
if [ "$READINESS_MODE" = "tag" ]; then
	if git rev-parse -q --verify "refs/tags/$RELEASE_VERSION" >/dev/null; then
		ok "local tag $RELEASE_VERSION exists for release workflow"
	else
		blocker "local tag $RELEASE_VERSION does not exist in release workflow"
	fi
elif git rev-parse -q --verify "refs/tags/$RELEASE_VERSION" >/dev/null; then
	blocker "local tag $RELEASE_VERSION already exists; tag creation requires explicit owner approval"
else
	ok "local tag $RELEASE_VERSION does not exist yet"
fi

if gh release view "$RELEASE_VERSION" --repo "$REPO" >/tmp/lazyss-release.$$.out 2>/tmp/lazyss-release.$$.err; then
	blocker "GitHub release $RELEASE_VERSION already exists; release action requires audit"
else
	ok "GitHub release $RELEASE_VERSION does not exist yet"
fi
rm -f /tmp/lazyss-release.$$.out /tmp/lazyss-release.$$.err

printf '\nHomebrew readiness\n'
set +e
"$ROOT/scripts/homebrew-readiness.sh" >/tmp/lazyss-homebrew-readiness.$$.out 2>&1
homebrew_rc=$?
set -e
case "$homebrew_rc" in
	0)
		ok "Homebrew readiness passed"
		;;
	2)
		blocker "Homebrew readiness has approval/external-state blockers"
		sed 's/^/  /' /tmp/lazyss-homebrew-readiness.$$.out
		;;
	*)
		fail "Homebrew readiness failed with exit $homebrew_rc"
		sed 's/^/  /' /tmp/lazyss-homebrew-readiness.$$.out
		;;
esac
rm -f /tmp/lazyss-homebrew-readiness.$$.out

printf '\nPrivate Homebrew install evidence\n'
check_homebrew_private_evidence

printf '\nLive smoke evidence\n'
check_live_smoke_evidence

printf '\nSummary\n'
if [ "$failures" -gt 0 ]; then
	printf '[fail] %s release readiness failure(s)\n' "$failures" >&2
	finish 1 failed
fi
if [ "$blockers" -gt 0 ]; then
	printf '[blocker] %s release readiness blocker(s)\n' "$blockers" >&2
	finish 2 blocked
fi

if [ "$warnings" -gt 0 ]; then
	printf '[warn] %s warning(s) reported\n' "$warnings"
fi

ok "release readiness audit passed"
write_reports ready
