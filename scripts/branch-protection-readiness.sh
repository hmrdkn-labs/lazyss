#!/usr/bin/env bash
set -euo pipefail

REPO="${LAZYSS_GITHUB_REPO:-hamardikan/lazyss}"
BRANCH="${LAZYSS_BRANCH_PROTECTION_BRANCH:-main}"
REQUIRED_CHECKS="${LAZYSS_REQUIRED_BRANCH_CHECKS:-ci-required}"

failures=0
blockers=0
warnings=0

ok() {
	printf '[ok] %s\n' "$*"
}

warn() {
	warnings=$((warnings + 1))
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

printf 'LazySS branch protection readiness audit\n'
printf 'repo: %s\n' "$REPO"
printf 'branch: %s\n' "$BRANCH"
printf 'mode: read-only; this script does not enable or modify branch protection\n\n'

need_command gh
need_command python3

if [ "$failures" -gt 0 ]; then
	printf '[fail] %s local readiness failure(s)\n' "$failures" >&2
	exit 1
fi

if ! gh api "repos/$REPO/branches/$BRANCH/protection" >/tmp/lazyss-branch-protection.$$.json 2>/tmp/lazyss-branch-protection.$$.err; then
	blocker "$BRANCH branch protection is not enabled or not visible; owner approval is required before enabling it"
	err="$(tr '\n' ' ' </tmp/lazyss-branch-protection.$$.err)"
	if [ -n "$err" ]; then
		warn "branch protection query detail: $err"
	fi
	rm -f /tmp/lazyss-branch-protection.$$.json /tmp/lazyss-branch-protection.$$.err
	printf '\nSummary\n'
	printf '[blocker] %s approval/external-state blocker(s)\n' "$blockers" >&2
	exit 2
fi
rm -f /tmp/lazyss-branch-protection.$$.err

set +e
validation_output="$(python3 - /tmp/lazyss-branch-protection.$$.json "$REQUIRED_CHECKS" <<'PY'
import json
import sys

path, required_checks_raw = sys.argv[1:]
required_checks = required_checks_raw.split()

with open(path, encoding="utf-8") as handle:
    data = json.load(handle)

def emit(level, message):
    print(f"{level}\t{message}")

def enabled_flag(name):
    value = data.get(name)
    return isinstance(value, dict) and value.get("enabled") is True

required_status_checks = data.get("required_status_checks")
if isinstance(required_status_checks, dict):
    emit("ok", "required status checks are enabled")
    if required_status_checks.get("strict") is True:
        emit("ok", "required branches must be up to date before merge")
    else:
        emit("blocker", "required status checks must set strict=true")

    configured = set()
    contexts = required_status_checks.get("contexts")
    if isinstance(contexts, list):
        configured.update(str(item) for item in contexts)
    checks = required_status_checks.get("checks")
    if isinstance(checks, list):
        for item in checks:
            if isinstance(item, dict) and item.get("context"):
                configured.add(str(item["context"]))

    missing = [name for name in required_checks if name not in configured]
    if missing:
        emit("blocker", "missing required status checks: " + ", ".join(missing))
    else:
        emit("ok", "required fast CI checks are protected: " + ", ".join(required_checks))
else:
    emit("blocker", "required status checks are not enabled")

if isinstance(data.get("required_pull_request_reviews"), dict):
    emit("ok", "pull request review requirement is enabled")
else:
    emit("blocker", "pull request review requirement is not enabled")

if enabled_flag("allow_force_pushes"):
    emit("blocker", "force pushes are allowed")
else:
    emit("ok", "force pushes are disabled")

if enabled_flag("allow_deletions"):
    emit("blocker", "branch deletion is allowed")
else:
    emit("ok", "branch deletion is disabled")

if not enabled_flag("enforce_admins"):
    emit("warn", "branch protection does not include administrators")

if not enabled_flag("required_linear_history"):
    emit("warn", "linear history is not required")
PY
)"
python_rc=$?
set -e
rm -f /tmp/lazyss-branch-protection.$$.json

if [ "$python_rc" -ne 0 ]; then
	fail "branch protection validator failed"
	printf '%s\n' "$validation_output" | sed 's/^/  /'
else
	while IFS=$'\t' read -r level message; do
		[ -n "$level" ] || continue
		case "$level" in
			ok) ok "$message" ;;
			warn) warn "$message" ;;
			blocker) blocker "$message" ;;
			fail) fail "$message" ;;
			*) fail "branch protection validator returned unknown level: $level" ;;
		esac
	done <<EOF
$validation_output
EOF
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

if [ "$warnings" -gt 0 ]; then
	warn "$warnings warning(s) reported"
fi

ok "branch protection readiness audit passed"
