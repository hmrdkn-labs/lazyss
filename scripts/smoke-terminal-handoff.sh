#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="${LAZYSS_BIN:-$ROOT/bin/lazyss}"
AWS_PROFILE="${LAZYSS_SMOKE_AWS_PROFILE:-${LAZYSS_LIVE_AWS_PROFILE:-}}"
AWS_REGION="${LAZYSS_SMOKE_AWS_REGION:-${LAZYSS_LIVE_AWS_REGION:-}}"

fail() {
	printf 'smoke-terminal-handoff: %s\n' "$*" >&2
	exit 1
}

note() {
	printf 'smoke-terminal-handoff: %s\n' "$*"
}

if ! command -v expect >/dev/null 2>&1; then
	fail "expect is required for terminal handoff smoke tests"
fi

cd "$ROOT"

tmpdir="$(mktemp -d)"
cleanup() {
	rm -rf "$tmpdir"
}
trap cleanup EXIT

assert_alt_exit_before_marker() {
	local log="$1"
	local marker="$2"
	perl -0e '
		my $marker = $ARGV[0];
		my $s = do { local $/; <STDIN> };
		my $exit = index($s, "\e[?1049l");
		my $child = index($s, $marker);
		die "missing alt-screen exit before $marker\n" if $exit < 0 || $child < 0 || $exit > $child;
	' "$marker" <"$log"
}

run_ssh_handoff() {
	local ssh_dir="$tmpdir/ssh"
	local log="$tmpdir/ssh.expect.log"
	mkdir -p "$ssh_dir/bin"

	cat >"$ssh_dir/bin/ssh" <<'EOF'
#!/usr/bin/env bash
printf 'LAZYSS_FAKE_SSH_START args=%s\n' "$*"
printf 'ssh-session$ '
IFS= read -r line
printf 'LAZYSS_FAKE_SSH_INPUT:%s\n' "$line"
exit 0
EOF
	chmod +x "$ssh_dir/bin/ssh"

	cat >"$ssh_dir/ssh_config" <<'EOF'
Host lazyss-handoff
  HostName 127.0.0.1
  User lazyss
  Port 22
EOF

	LOG="$log" \
	PATH="$ssh_dir/bin:$PATH" \
	LAZYSS_SMOKE_BIN="$BIN" \
	LAZYSS_SMOKE_CONFIG="$ssh_dir/ssh_config" \
	expect <<'EOF'
log_user 0
log_file -a -noappend $env(LOG)
set stty_init "rows 40 columns 120"
set timeout 10
spawn env TERM=xterm PATH=$env(PATH) $env(LAZYSS_SMOKE_BIN) --source ssh --ssh-config $env(LAZYSS_SMOKE_CONFIG)
expect {
  "lazyss-handoff" {}
  timeout {
    puts stderr "timed out waiting for SSH handoff inventory"
    exit 1
  }
}
send "\r"
expect "LAZYSS_FAKE_SSH_START"
expect "ssh-session$ "
send "whoami\r"
expect "LAZYSS_FAKE_SSH_INPUT:whoami"
expect "session ended"
send "q"
expect eof
set result [wait]
exit [lindex $result 3]
EOF

	assert_alt_exit_before_marker "$log" "LAZYSS_FAKE_SSH_START"
	note "SSH handoff passed"
}

run_aws_handoff() {
	if [ -z "$AWS_PROFILE" ]; then
		note "AWS SSM handoff skipped; set LAZYSS_SMOKE_AWS_PROFILE to enable"
		return
	fi

	local aws_dir="$tmpdir/aws"
	local log="$tmpdir/aws.expect.log"
	mkdir -p "$aws_dir/bin"

	cat >"$aws_dir/bin/aws" <<'EOF'
#!/usr/bin/env bash
printf 'LAZYSS_FAKE_AWS_START args=%s\n' "$*"
printf 'ssm-session$ '
IFS= read -r line
printf 'LAZYSS_FAKE_AWS_INPUT:%s\n' "$line"
exit 0
EOF
	chmod +x "$aws_dir/bin/aws"

	local -a args=(--source aws --aws-profile "$AWS_PROFILE")
	if [ -n "$AWS_REGION" ]; then
		args+=(--aws-region "$AWS_REGION")
	fi

	LOG="$log" \
	PATH="$aws_dir/bin:$PATH" \
	LAZYSS_SMOKE_BIN="$BIN" \
	LAZYSS_AWS_ARGS="${args[*]}" \
	expect <<'EOF'
log_user 0
log_file -a -noappend $env(LOG)
set stty_init "rows 40 columns 140"
set timeout 30
eval spawn env TERM=xterm PATH=$env(PATH) $env(LAZYSS_SMOKE_BIN) $env(LAZYSS_AWS_ARGS)
expect {
  "aws-ssm-shell" {}
  "source aws degraded" {
    puts stderr "aws inventory degraded before handoff"
    exit 2
  }
  timeout {
    puts stderr "timed out waiting for AWS SSM inventory"
    exit 1
  }
}
send "\r"
expect "LAZYSS_FAKE_AWS_START"
expect "ssm-session$ "
send "uptime\r"
expect "LAZYSS_FAKE_AWS_INPUT:uptime"
expect "session ended"
send "q"
expect eof
set result [wait]
exit [lindex $result 3]
EOF

	assert_alt_exit_before_marker "$log" "LAZYSS_FAKE_AWS_START"
	note "AWS SSM handoff passed"
}

run_ssh_handoff
run_aws_handoff
note "terminal handoff smoke passed"
