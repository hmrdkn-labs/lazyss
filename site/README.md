# lazyss-site

The page at https://lazyss.hamardikan.com/: one static page that prints a recorded `G` health sweep of
lazyss the way a teleprinter would have. Deployed as a Workers static-assets site (no Worker script, no
bindings). Concept, reference, claims and self-critique are in `NOTES.md`.

## Layout

- `public/`: the site. `index.html` holds all the copy, and between marker comments (`sweep:`,
  `signals:`, `build:`) the parts the build writes. `style.css`, `sweep.js` (the only script, deferred),
  `404.html`, `_headers`, `robots.txt`, `sitemap.xml`, `favicon.svg`, `apple-touch-icon.png`, `og.png`.
- `public/assets/`: self-hosted fonts (IBM Plex Mono subset, Source Serif 4 Latin, both OFL, licence texts
  beside them) and `tractor.svg`, the mask for the paper's feed holes. File names carry no hash: rename a
  file when its content changes (`/assets/*` is cached as immutable).
- `public/data/signals.json`: the GitHub snapshot (release, formula version, CI on main, licence). Generated.
- `public/data/replay.json`: the sweep frames `sweep.js` replays. Generated from `data/sweep.json`.
- `data/sweep.json`: the recorded sweep (frames from tmux, with their `t_ms`). Written by
  `scripts/capture-sweep.sh`, never by hand.
- `scripts/build.sh`: snapshots the signals with `gh`, then runs `scripts/render.mjs`, which pre-renders the
  final sweep frame (100 and 60 columns) and the signals into `public/index.html`, and writes the footer's
  deploy sha and build date.
- `scripts/og.mjs`: redraws `public/og.png` and `public/apple-touch-icon.png` (needs Playwright).
- `scripts/verify.mjs`: the checks and the shots (needs Playwright).

## Build and deploy

```sh
npm install
npm run build        # SKIP_GITHUB=1 npm run build keeps the committed public/data/signals.json
npm run dev          # build, then wrangler dev --local
npm run deploy       # build, then wrangler deploy (custom domain route is in wrangler.jsonc)
```

The build needs `gh` (authenticated), `jq` and `node`. A signal that cannot be read is written as `null` and
the page prints "No data". The snapshot is written to a temp file and moved only when `jq -e` accepts it.
Commit `public/index.html`, `public/data/*.json` and the images after a build; the footer sha is the commit
the build ran on, so build again after committing if the footer must name the final commit.

To re-record the sweep: `bash scripts/capture-sweep.sh` (needs tmux and Go), then build.

Playwright is not a dependency. Install it once outside the repo:

```sh
npm i --prefix ~/.cache/lazyss-pw playwright @axe-core/playwright
npx --prefix ~/.cache/lazyss-pw playwright install chromium
node scripts/verify.mjs [shots-dir]
python3 -m http.server 8791 -d public & node scripts/og.mjs
```

## Things to keep true

- The sweep is a recording, and the caption says so: capture date, lazyss version, host count and
  "synthetic SSH config" come from `data/sweep.json`. The build refuses a recording that does not say
  synthetic, shows hosts still checking in its final frame, or has no host lines.
- No IP address, port, host name, path or account value in the page, its comments or `data/`. The
  capture uses invented aliases only. Check before publishing:
  `rg -n '\b\d{1,3}(\.\d{1,3}){3}\b|/Users/|/home/' public data --glob '!assets'`.
- Every sentence of behaviour on the page is a row in the claims table in `NOTES.md` with its source.
  Unresolved facts (smoke-test status, the CV timing figure, stars) stay off the page.
- The page works with JavaScript off and under `prefers-reduced-motion`: the finished frame is in the HTML
  and the replay and Copy controls are added by `sweep.js` only.
- The CSP in `_headers` allows `'self'` only. No inline script or style, no analytics, no third-party
  request. `sweep.js` fetches `/data/replay.json` from the same origin.
- Colours are two inks on paper (red for what failed). Every status is a word as well as a colour.
- Under 1280 px the 60-column recording is shown; from 1280 px the 100-column one. Text is never scaled.
