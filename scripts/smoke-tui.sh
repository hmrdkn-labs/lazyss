#!/usr/bin/env bash
set -euo pipefail

# tmux-driven E2E for the cockpit TUI. No AWS/network dependency: it drives a
# synthetic SSH config through the ssh source and redirects HOME/XDG so real
# operator state (state.json) and ~/.ssh are never read or written.

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="${LAZYSS_BIN:-$ROOT/bin/lazyss}"
KEYMAP="$ROOT/internal/tui/keymap.go"
LONGHOST="prod-web-frontend-primary-node-useast-1a-alpha01" # 48 chars, exercises untruncated NAME at 140

FAILS=0
SESSIONS=()

note() { printf 'smoke-tui: %s\n' "$*"; }
pass() { printf 'PASS: %s\n' "$*"; }
fail() {
	printf 'FAIL: %s\n' "$*"
	FAILS=$((FAILS + 1))
}

if ! command -v tmux >/dev/null 2>&1; then
	printf 'smoke-tui: tmux is required\n' >&2
	exit 1
fi
if ! command -v perl >/dev/null 2>&1; then
	printf 'smoke-tui: perl is required\n' >&2
	exit 1
fi

cd "$ROOT"
note "building binary"
make build >/dev/null

tmpdir="$(mktemp -d)"
cleanup() {
	for s in "${SESSIONS[@]:-}"; do
		[ -n "$s" ] && tmux kill-session -t "$s" 2>/dev/null || true
	done
	rm -rf "$tmpdir"
}
trap cleanup EXIT

ssh_config="$tmpdir/ssh_config"
cat >"$ssh_config" <<EOF
Host db
  HostName 10.0.0.5
  User ops
  Port 22

Host $LONGHOST
  HostName 10.0.0.9
  User deploy
  Port 22

Host github.com
  HostName github.com
  User git

Host dup-a
  HostName 10.0.0.42
  User ops
  Port 22

Host dup-b
  HostName 10.0.0.42
  User ops
  Port 22

Host tunnel-pg
  HostName 127.0.0.1
  User ops
  LocalForward 5432 db.internal:5432
EOF
cp "$ssh_config" "$ssh_config.before"

# Footer/help tokens come straight from the registry so the assertion tracks the
# source of truth instead of a hand-maintained copy.
TOKENS=()
while IFS= read -r tok; do TOKENS+=("$tok"); done < <(perl -ne 'print "$1 $2\n" if /label:\s*"([^"]*)",\s*desc:\s*"([^"]*)"/' "$KEYMAP")
if [ "${#TOKENS[@]}" -lt 2 ]; then
	printf 'smoke-tui: could not parse key tokens from %s\n' "$KEYMAP" >&2
	exit 1
fi

launch() {
	# launch <session> <width> <height>
	local session="$1" w="$2" h="$3"
	SESSIONS+=("$session")
	tmux kill-session -t "$session" 2>/dev/null || true
	tmux new-session -d -s "$session" -x "$w" -y "$h" \
		"HOME=$tmpdir/home XDG_CONFIG_HOME=$tmpdir/home/.config $BIN --source ssh --ssh-config $ssh_config"
	# Wait for the async inventory fetch to render the cockpit (cursor marker).
	local i
	for i in $(seq 1 40); do
		if tmux capture-pane -t "$session" -p 2>/dev/null | grep -q '›'; then
			return 0
		fi
		sleep 0.25
	done
	return 1
}

snap() { tmux capture-pane -t "$1" -p >"$2"; }

send() {
	tmux send-keys -t "$1" "$2"
	sleep 0.5
}

count_char() {
	# count_char <hex-codepoint>; counts occurrences on stdin
	perl -CS -e 'my $cp=shift; my $n=0; while(<STDIN>){ my @m = /\x{$cp}/g; $n += @m } print $n' "$1"
}

assert_contains() {
	# assert_contains <file> <needle> <desc>
	if grep -qF -- "$2" "$1"; then pass "$3"; else fail "$3"; fi
}
assert_absent() {
	if grep -qF -- "$2" "$1"; then fail "$3"; else pass "$3"; fi
}

assert_no_overflow() {
	# assert_no_overflow <file> <width> <desc>
	if perl -CS -e 'my $w=shift; my $bad=0; while(<STDIN>){ chomp; if(length>$w){ printf STDERR "  overlong(%d>%d): %s\n", length,$w,$_; $bad=1 } } exit($bad)' "$2" <"$1"; then
		pass "$3"
	else
		fail "$3"
	fi
}

assert_split_borders() {
	# assert_split_borders <file> <desc>: both panels share one top row and one
	# bottom row => each border char appears exactly twice on its row.
	local cap="$1" desc="$2" top bot line n
	top="$(grep -n '╭' "$cap" | head -1 | cut -d: -f1 || true)"
	bot="$(grep -n '╰' "$cap" | tail -1 | cut -d: -f1 || true)"
	if [ -z "$top" ] || [ -z "$bot" ] || [ "$top" = "$bot" ]; then
		fail "$desc (no distinct top/bottom border rows)"
		return
	fi
	line="$(sed -n "${top}p" "$cap")"
	n="$(printf '%s' "$line" | count_char 256d)" # ╭
	if [ "$n" -ne 2 ]; then
		fail "$desc (top row has $n left-corners, want 2)"
		return
	fi
	n="$(printf '%s' "$line" | count_char 256e)" # ╮
	[ "$n" -eq 2 ] || {
		fail "$desc (top row has $n right-corners, want 2)"
		return
	}
	line="$(sed -n "${bot}p" "$cap")"
	n="$(printf '%s' "$line" | count_char 2570)" # ╰
	[ "$n" -eq 2 ] || {
		fail "$desc (bottom row has $n left-corners, want 2)"
		return
	}
	n="$(printf '%s' "$line" | count_char 256f)" # ╯
	[ "$n" -eq 2 ] || {
		fail "$desc (bottom row has $n right-corners, want 2)"
		return
	}
	pass "$desc"
}

assert_footer() {
	# assert_footer <file> <desc-prefix>: every registered token appears in full
	# (a mid-token clip would break the exact substring) and the last rendered
	# line ends with the final token 'q quit'.
	local cap="$1" prefix="$2" tok missing=0 last
	for tok in "${TOKENS[@]}"; do
		grep -qF -- "$tok" "$cap" || {
			printf '  missing footer token: %q\n' "$tok"
			missing=1
		}
	done
	if [ "$missing" -eq 0 ]; then pass "$prefix footer tokens complete"; else fail "$prefix footer tokens complete"; fi
	last="$(awk 'NF{l=$0} END{print l}' "$cap")"
	if [[ "$last" == *"q quit" ]]; then pass "$prefix footer ends with 'q quit' (no clip)"; else fail "$prefix footer ends with 'q quit' (no clip)"; fi
}

# --- per-size layout assertions ---------------------------------------------
for spec in "140 38" "100 30" "80 24"; do
	set -- $spec
	W="$1"
	H="$2"
	session="lazyss-smoke-${W}x${H}"
	note "size ${W}x${H}"
	if ! launch "$session" "$W" "$H"; then
		fail "${W}x${H}: cockpit rendered"
		continue
	fi
	cap="$tmpdir/cap-${W}x${H}.txt"
	snap "$session" "$cap"

	assert_contains "$cap" "›" "${W}x${H}: selection marker present"
	assert_footer "$cap" "${W}x${H}:"
	assert_no_overflow "$cap" "$W" "${W}x${H}: no line exceeds width"
	if [ "$W" -ge 92 ]; then
		assert_split_borders "$cap" "${W}x${H}: split panel borders aligned"
	fi
	if [ "$W" -ge 140 ]; then
		assert_contains "$cap" "$LONGHOST" "${W}x${H}: 40+ char hostname untruncated"
	fi
	tmux kill-session -t "$session" 2>/dev/null || true
done

# --- interactions at 140x38 --------------------------------------------------
note "interactions 140x38"
session="lazyss-smoke-interact"
if launch "$session" 140 38; then
	icap="$tmpdir/cap-interact.txt"

	send "$session" "?"
	snap "$session" "$icap"
	assert_contains "$icap" "cycle access method" "help overlay lists m"
	assert_contains "$icap" "copy connect command" "help overlay lists c"
	send "$session" "Escape"
	snap "$session" "$icap"
	assert_absent "$icap" "toggle this help" "esc closes help overlay"

	send "$session" "e"
	snap "$session" "$icap"
	assert_contains "$icap" "Edit overlay" "editor opens on e"
	send "$session" "Escape"
	snap "$session" "$icap"
	assert_absent "$icap" "Edit overlay" "esc closes editor"

	send "$session" "v"
	snap "$session" "$icap"
	assert_contains "$icap" "History:" "history opens on v"
	send "$session" "Escape"
	snap "$session" "$icap"
	assert_absent "$icap" "History:" "esc closes history"

	send "$session" "C"
	snap "$session" "$icap"
	assert_contains "$icap" "Delete candidates" "cleanup lists delete candidates"
	assert_contains "$icap" "duplicate target" "cleanup shows duplicate-target candidate"
	assert_contains "$icap" "github.com" "cleanup shows github.com identity"
	assert_contains "$icap" "Protected (kept, not selectable)" "cleanup marks github.com protected"
	send "$session" "Escape"
	snap "$session" "$icap"
	assert_absent "$icap" "Delete candidates" "esc closes cleanup"

	# h hides the detail panel: the second bordered panel (Details) disappears
	# and the list spans full width.
	snap "$session" "$icap"
	assert_contains "$icap" "Details" "detail panel shown by default"
	send "$session" "h"
	snap "$session" "$icap"
	assert_absent "$icap" "Details" "h hides detail panel"
	send "$session" "h"
	snap "$session" "$icap"
	assert_contains "$icap" "Details" "h restores detail panel"

	tmux kill-session -t "$session" 2>/dev/null || true
else
	fail "interactions: cockpit rendered"
fi

# --- config must be untouched ------------------------------------------------
if cmp -s "$ssh_config" "$ssh_config.before"; then
	pass "fixture SSH config unmodified"
else
	fail "fixture SSH config unmodified"
fi

if [ "$FAILS" -ne 0 ]; then
	note "$FAILS assertion(s) failed"
	exit 1
fi
note "all assertions passed"
