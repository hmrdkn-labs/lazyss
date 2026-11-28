# Brief: site-v3 (lazyss.hamardikan.com after v0.3.0)

Worktree: `~/workspace/.worktrees/lazyss/site-v3`, branch `feat/site-v3`, based on `main` after v0.3.0 is
released. Read first, in full: `site/README.md` (especially "Things to keep true"), `site/NOTES.md`
(claims table, SEO section), `site/public/index.html`, `site/public/style.css`, `site/scripts/audit.mjs`,
`~/workspace/sandbox/orchestration-2026-11-28/siblings.md`, and the released code you will describe:
`README.md`, `internal/adapters/gcloud`, `internal/adapters/azcli`, `cmd/lazyss/main.go`, `internal/tui/keymap.go`.

## Owner decisions (do not reopen)

- The page says lazyss covers SSH, AWS SSM, Google Cloud and Azure.
- Every new sentence of behaviour gets a row in the `site/NOTES.md` claims table, backed by released v0.3.0
  code (cite `file:symbol` at tag `v0.3.0`).
- The "More tools by hmrdkn-labs" section becomes a real section near the footer, one-line pitch each,
  wording copied from `siblings.md` (heading, order, rules there). lazyss itself is left out of its own page.

## Files you own

`site/` (except `site/data/sweep.json`, see below), `~/workspace/sandbox/orchestration-2026-11-28/siblings.md`
(only the lazyss row: update its pitch to cover Google Cloud and Azure, condensed from the new meta
description, and its "Source" cell), `docs/briefs/site-v3.md` (Report).

## What to do

1. Hero, `<title>`, meta description, JSON-LD, og/twitter tags: SSH, AWS SSM, Google Cloud and Azure.
   Keep the SEO constraints from `NOTES.md` (lengths, one h1). Redraw `og.png` with `scripts/og.mjs` only if
   it names providers.
2. A providers section: one short entry per provider with how it connects (the exact command lazyss runs,
   e.g. `gcloud compute ssh … --tunnel-through-iap`, `az network bastion ssh …`), that it reuses the cloud
   CLI's login and never stores tokens, and what the health check proves (TCP port open ≠ login works;
   IAP/Bastion = provider state only). No IPs, hosts, paths or account values (README rule).
3. "More tools by hmrdkn-labs" section near the footer, per `siblings.md`. Remove the old footer-line duplicate
   if one exists, so the links appear once. Style with existing tokens only (no new colours, shadows,
   gradients); type sizes per README.
4. Claims table rows for every new sentence; update "Real vs missing" if needed.
5. Sweep recording: it shows a synthetic SSH config sweep. Re-record with `scripts/capture-sweep.sh` only if
   the page copy around it would now mislead (for example if the caption implies it shows every provider).
   Prefer fixing the caption. State your decision in the Report.
6. `SKIP_GITHUB=0 npm run build` (needs gh), `node scripts/audit.mjs` (Playwright per README), the privacy
   `rg` from README. Do NOT run `npm run deploy`: the orchestrator asks the owner and deploys.

## Checks to pass before reporting

Audit passes (Lighthouse included), privacy rg returns nothing, build clean, `make check` unaffected.
Conventional commits (`feat(site): ...`, `build(site): ...`). Do not push or deploy.

## Process safety (applies to every brief)

Never kill processes by name (`pkill -f`, `killall`, e.g. wrangler/workerd): other agents run production
processes on this Mac. Start any dev server (`npm run dev`, `wrangler dev`, `python3 -m http.server`) with
`cmd & echo $! > <pidfile>` and kill only that PID and its children. Do not use port 8787 (taken by another
agent); pick another free port.

Every Chrome/Chromium launch must pass `--use-mock-keychain --password-store=basic`, or macOS shows
keychain prompts to the owner. Before running any script, make that true in `site/scripts/audit.mjs`
(`chromium.launch({ args: [...] })` and Lighthouse's `--chrome-flags`), `site/scripts/shots.mjs` and
`site/scripts/og.mjs`, as its own commit (`fix(site): ...`).

## Report (fill in, then reply in ≤ 20 lines)

- Commits:
- Audit result (scores):
- Sweep re-recorded? Why / why not:
- New claims rows (count) and any claim you dropped for lack of evidence:
- siblings.md lazyss pitch, final wording:
