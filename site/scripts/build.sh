#!/usr/bin/env bash
# Prepares public/ for a deploy:
#   1. snapshots the living signals into public/data/signals.json with `gh` (needs an authenticated gh, and jq);
#   2. pre-renders the recorded sweep (data/sweep.json) and the signals into public/index.html, and writes
#      public/data/replay.json for sweep.js (scripts/render.mjs);
#   3. the footer's deploy sha and build date are written by the same step.
# SKIP_GITHUB=1 keeps the committed public/data/signals.json. The page draws from the snapshot only.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DATA="$ROOT/public/data"
REPO="${LAZYSS_REPO:-hmrdkn-labs/lazyss}"
TAP="${LAZYSS_TAP:-hmrdkn-labs/homebrew-tap}"
mkdir -p "$DATA"

if [ "${SKIP_GITHUB:-0}" = "1" ]; then
  echo "SKIP_GITHUB=1: keeping public/data/signals.json"
else
  # each signal is read on its own: a failure writes null and the page prints "No data"
  release="$(gh release list --repo "$REPO" --limit 1 --json tagName,publishedAt 2>/dev/null \
    | jq -c --arg repo "$REPO" '.[0] | select(.) | {tag: .tagName, published_at: .publishedAt, url: "https://github.com/\($repo)/releases/tag/\(.tagName)"}' 2>/dev/null || true)"
  formula="$(gh api "repos/$TAP/contents/Formula/lazyss.rb" --jq .content 2>/dev/null \
    | base64 -d 2>/dev/null | sed -n 's/^[[:space:]]*version "\([^"]*\)".*/\1/p' | head -n1 | jq -R -c 'select(length > 0)' 2>/dev/null || true)"
  ci="$(gh run list --repo "$REPO" --branch main --event push --limit 1 --json conclusion,createdAt,url 2>/dev/null \
    | jq -c '.[0] | select(.) | select(.conclusion != "") | {conclusion, created_at: .createdAt, url}' 2>/dev/null || true)"
  license="$(gh api "repos/$REPO" --jq .license.spdx_id 2>/dev/null | jq -R -c 'select(length > 0 and . != "NOASSERTION")' 2>/dev/null || true)"

  # written beside the target and moved only when complete and valid, so a failed run cannot leave a broken file
  TMP="$(mktemp "$DATA/.signals.json.XXXXXX")"
  trap 'rm -f "$TMP"' EXIT
  jq -n --arg at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --argjson release "${release:-null}" --argjson formula "${formula:-null}" \
    --argjson ci "${ci:-null}" --argjson license "${license:-null}" \
    '{generated_at: $at, release: $release, formula: $formula, ci: $ci, license: $license}' > "$TMP"
  jq -e '.generated_at and has("release") and has("formula") and has("ci") and has("license")' "$TMP" > /dev/null
  chmod 644 "$TMP"
  mv "$TMP" "$DATA/signals.json"
  trap - EXIT
fi

node "$ROOT/scripts/render.mjs"
