#!/usr/bin/env bash
set -euo pipefail

# Captures a real `G` (health check all) sweep of the cockpit in tmux against a
# synthetic SSH inventory and writes site/data/sweep.json. Requires tmux, python3,
# go, ssh. HOME/XDG are redirected so real ~/.ssh, AWS config and state.json are
# never read or written; no AWS calls, no real hosts. Results come from real TCP
# dials against throwaway loopback listeners, closed loopback ports, a
# documentation address (192.0.2.0/24, dropped) and a .invalid name.

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CAP="${CAPTURE_DIR:-$HOME/workspace/sandbox/showcase-scan/build/lazyss/capture}"
OUT="${SWEEP_OUT:-$ROOT/site/data/sweep.json}"
HELPER="$ROOT/site/scripts/frames-to-json.py"
SOCK="lazyss-capture-$$"
SESSION="cap"

for tool in tmux python3 go ssh; do
	command -v "$tool" >/dev/null || { echo "capture-sweep: $tool is required" >&2; exit 1; }
done

cd "$ROOT"
make build >/dev/null
BIN="$ROOT/bin/lazyss"
VERSION_OUT="$("$BIN" --version)"

tmpdir="$(mktemp -d)"
cleanup() {
	tmux -L "$SOCK" kill-server 2>/dev/null || true
	# match on this run's unique --directory so no one else's listeners are touched
	pkill -f -- "--directory $tmpdir/www" 2>/dev/null || true
	rm -rf "$tmpdir"
}
trap cleanup EXIT

rm -rf "$CAP/frames" "$CAP"/extra-*.ansi
mkdir -p "$CAP/frames/100" "$CAP/frames/60" "$tmpdir/home/.config" "$tmpdir/bin" "$tmpdir/www"

# --- loopback ports: 5 listeners + 2 closed ----------------------------------
read -r -a PORTS < <(python3 - <<'PY'
import socket
socks = [socket.socket() for _ in range(7)]
for s in socks: s.bind(("127.0.0.1", 0))
print(" ".join(str(s.getsockname()[1]) for s in socks))
PY
)
for i in 0 1 2 3 4; do
	python3 -m http.server "${PORTS[$i]}" --bind 127.0.0.1 --directory "$tmpdir/www" </dev/null >/dev/null 2>&1 &
	disown "$!"
done
sleep 1

# --- synthetic inventory (10 hosts) -------------------------------------------
cfg="$tmpdir/ssh_config"
host() { # alias hostname port
	printf 'Host %s\n  HostName %s\n  User ops\n  Port %s\n\n' "$1" "$2" "$3"
}
{
	host web-1 web-1.localhost "${PORTS[0]}"
	host web-2 web-2.localhost "${PORTS[1]}"
	host api api.localhost "${PORTS[2]}"
	host queue queue.localhost "${PORTS[3]}"
	host db-primary db-primary.localhost "${PORTS[4]}"
	host db-replica db-replica.localhost "${PORTS[5]}"  # closed port: refused
	host cache cache.localhost "${PORTS[6]}"            # closed port: refused
	host build 192.0.2.10 22                            # documentation address: dropped, times out
	host jump 192.0.2.20 22                             # documentation address: dropped, times out
	host bastion-old bastion-old.invalid 22             # does not resolve
} >"$cfg"
cp "$cfg" "$cfg.before"

# The checker runs `ssh -G <alias>`, which reads the real ~/.ssh/config of the
# account (OpenSSH ignores $HOME). Shim ssh so it resolves against the synthetic
# config instead.
real_ssh="$(command -v ssh)"
printf '#!/bin/sh\nexec %s -F %s "$@"\n' "$real_ssh" "$cfg" >"$tmpdir/bin/ssh"
chmod +x "$tmpdir/bin/ssh"

# TERM must not start with tmux/screen or colorprofile ignores COLORTERM=truecolor.
run_env="HOME=$tmpdir/home XDG_CONFIG_HOME=$tmpdir/home/.config XDG_STATE_HOME=$tmpdir/home/.state XDG_DATA_HOME=$tmpdir/home/.data XDG_CACHE_HOME=$tmpdir/home/.cache PATH=$tmpdir/bin:$PATH COLORTERM=truecolor TERM=xterm-256color"

printf 'set -g default-terminal tmux-256color\nset -ga terminal-features ",*:RGB"\nset -g status off\n' >"$tmpdir/tmux.conf"

start() { # cols rows
	tmux -L "$SOCK" kill-server 2>/dev/null || true
	rm -rf "$tmpdir/home/Library" "$tmpdir/home/.state" "$tmpdir/home/.data" "$tmpdir/home/.config/lazyss"
	tmux -L "$SOCK" -f "$tmpdir/tmux.conf" new-session -d -s "$SESSION" -x "$1" -y "$2" \
		"env -u NO_COLOR $run_env $BIN --source ssh --ssh-config $cfg" </dev/null >/dev/null 2>&1
	local i
	for i in $(seq 1 60); do
		tmux -L "$SOCK" capture-pane -t "$SESSION" -p | grep -q '›' && return 0
		sleep 0.25
	done
	echo "capture-sweep: cockpit did not render" >&2
	return 1
}
key() { tmux -L "$SOCK" send-keys -t "$SESSION" "$1"; sleep 0.4; }
poll() { python3 "$HELPER" poll --socket "$SOCK" --session "$SESSION" "$@"; }
snap() { python3 "$HELPER" snap --socket "$SOCK" --session "$SESSION" --out "$1"; }

# A sweep can finish between two samples and skip the "checking 0/10" frame, so
# retry until at least 4 distinct frames were seen.
sweep() { # cols [pre-key]
	local attempt n
	for attempt in 1 2 3 4 5; do
		start "$1" 24
		[ -n "${2:-}" ] && key "$2"
		rm -rf "$CAP/frames/$1" && mkdir -p "$CAP/frames/$1"
		poll --outdir "$CAP/frames/$1" --press G --wait-while checking
		n="$(ls "$CAP/frames/$1" | wc -l)"
		[ "$n" -ge 4 ] && return 0
	done
	echo "capture-sweep: only $n frames at $1 columns" >&2
	return 1
}

# 100x24: hide the detail panel (it prints HostName and the checked port), sweep.
sweep 100 h

# 60x24: compact list, no detail panel at this width.
sweep 60

# Help overlay at 100 columns. The detail panel is not captured: it prints the
# HostName (and, after a check, host:port), which must not appear in kept frames.
start 100 24
key '?'
snap "$CAP/extra-help.ansi"

tmux -L "$SOCK" kill-server 2>/dev/null || true
cmp -s "$cfg" "$cfg.before" || { echo "capture-sweep: synthetic config changed" >&2; exit 1; }

cat >"$tmpdir/meta.json" <<JSON
{
 "captured_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
 "lazyss_version": "$VERSION_OUT",
 "command": "lazyss --source ssh --ssh-config <synthetic>",
 "inventory": "synthetic SSH config, 10 hosts, local listeners",
 "_sizes": {"100": 24, "60": 24}
}
JSON
mkdir -p "$(dirname "$OUT")"
python3 "$HELPER" build --capture-dir "$CAP" --out "$OUT" --meta "$tmpdir/meta.json"
echo "capture-sweep: wrote $OUT"
