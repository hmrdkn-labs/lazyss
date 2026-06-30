#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="${LAZYSS_BIN:-$ROOT/bin/lazyss}"
REGION="${LAZYSS_SMOKE_AWS_REGION:-us-east-1}"

fail() {
	printf 'smoke-local: %s\n' "$*" >&2
	exit 1
}

note() {
	printf 'smoke-local: %s\n' "$*"
}

if ! command -v expect >/dev/null 2>&1; then
	fail "expect is required for the TUI PTY smoke test"
fi

cd "$ROOT"

note "building binary"
make build >/dev/null

version_output="$("$BIN" --version)"
case "$version_output" in
lazyss\ *) ;;
*) fail "unexpected version output: $version_output" ;;
esac
note "$version_output"

tmpdir="$(mktemp -d)"
cleanup() {
	rm -rf "$tmpdir"
}
trap cleanup EXIT

doctor_output="$tmpdir/doctor.out"
set +e
AWS_EC2_METADATA_DISABLED=true "$BIN" doctor --aws-region "$REGION" >"$doctor_output" 2>&1
doctor_rc=$?
set -e

grep -q "lazyss doctor" "$doctor_output" || fail "doctor header missing"
grep -Eq "\\[(ok|fail)\\] ssh" "$doctor_output" || fail "doctor ssh check missing"
if grep -Eiq 'AWS_SECRET_ACCESS_KEY|AWS_SESSION_TOKEN|BEGIN (OPENSSH|RSA|EC|DSA) PRIVATE KEY|aws_secret_access_key|aws_session_token' "$doctor_output"; then
	fail "doctor output appears to contain credential material"
fi
note "doctor completed with exit $doctor_rc"

ssh_config="$tmpdir/ssh_config"
cat >"$ssh_config" <<'EOF'
Host lazyss-smoke
  HostName 127.0.0.1
  User lazyss-smoke
  Port 22
EOF
cp "$ssh_config" "$ssh_config.before"

LAZYSS_SMOKE_BIN="$BIN" LAZYSS_SMOKE_CONFIG="$ssh_config" expect <<'EOF'
log_user 0
set stty_init "rows 40 columns 120"
set timeout 10
spawn env TERM=xterm $env(LAZYSS_SMOKE_BIN) --source ssh --ssh-config $env(LAZYSS_SMOKE_CONFIG)
expect {
  "Lazy Secure Shell" {}
  timeout {
    puts stderr "timed out waiting for Lazy Secure Shell header"
    exit 1
  }
}
expect {
  "lazyss-smoke" {}
  timeout {
    puts stderr "timed out waiting for temp SSH inventory row"
    exit 1
  }
}
send "q"
expect eof
set result [wait]
exit [lindex $result 3]
EOF

cmp -s "$ssh_config" "$ssh_config.before" || fail "temporary SSH config was mutated"
note "TUI rendered temp SSH inventory without mutating config"
note "local smoke passed"
