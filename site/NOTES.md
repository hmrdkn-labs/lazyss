# LazySS landing page: notes

Live: https://lazyss.hamardikan.com/ · Source: `site/` · Worker: `lazyss-site`.

## Concept

**Hard copy.** Before a terminal was a window it was a teleprinter: a keyboard and a roll of paper.
Every command you ran left a printed line. lazyss is a terminal program whose most telling moment is
a sweep: press `G` and one line per host comes back as each check finishes. The page prints that
sweep the way a teleprinter would have, line by line, at the times the real run returned them.

### Reference

The Teletype Model 33 ASR console log, and IBM fanfold listing paper (14 7/8 × 11 in, tractor-feed
margins). Honoured as:

- **Grid:** the page is a listing. A narrow left margin (the tractor strip, drawn once as an SVG
  hole pattern beside the capture only), a fixed-character measure for the capture, a 64ch
  measure for prose. Section labels sit in the margin like listing column heads, not as banners.
- **Type:** one monospace for the capture, keys and commands (IBM Plex Mono, OFL); one serif for
  prose (Source Serif 4, OFL). Two families. No Inter or JetBrains Mono.
- **Colour:** paper and a two-colour ribbon. Teleprinter ribbons were black and red; red printed
  exceptions. Light: paper `#f5f2ea`, ink `#1c1b19`, ribbon red `#a3271f`. Dark: carbon
  `#1b1a18`, ink `#ebe6da`, red `#ff8a7a`. Every pair checked to AA; status is always a word too,
  never colour alone.
- **Texture:** none painted. Rules are hairlines, the way listing paper has ruled bands. No
  gradients, glow, shadows or rounded cards.

## Signature object

The `G` health sweep (research §3), rendered from `data/sweep.json`: real frames captured with tmux
from `bin/lazyss --source ssh` against a synthetic SSH config in a temp `HOME`
(`site/scripts/capture-sweep.sh`). Text and SVG, not an image, not a window.

- Desktop (≥1280 px): the 100-column capture. Narrower: the 60-column capture in a box that
  scrolls sideways with a visible scrollbar; text is never scaled down.
- **Signature motion:** "print". When the reader presses "Run the sweep again" (and once on first view
  with motion allowed), frames advance at their real `t_ms` offsets; a line that changes gets a
  short ink-darken (opacity 0.55 → 1, 180 ms), like a fresh strike of the ribbon. No typing cursor.
  The whole run is under 10 s and does not loop.
- **Static fallback:** the finished frame is pre-rendered into `index.html` at build time. With
  JavaScript off or `prefers-reduced-motion: reduce`, that is what shows; the replay button is not
  rendered.
- **Micro-interactions (3):** (1) each host's result time ("+0.3 s") prints in the margin at
  ≥1280 px, or in a list "In the order they came back" under the 60-column capture, as that row's
  result lands in the replay; static text, not a control (review round 1 removed interactive host
  rows: 20 px targets, and a note that disagreed with the replayed frame); (2) "Copy" beside the
  brew command says "Copied" for 2 s, with a live region; (3) the replay control shows elapsed time
  against the real run length.

## Page outline

1. Header: name, one line of what it is (h1), install line, link to v0.2.0 release notes.
2. The sweep, with its caption: what was run, when, against what (synthetic), and that this is a
   recording, not a live feed.
3. What it checks · What it reads · Cleanup · Install (short, from research §4, verified below).
4. Signals: latest release, formula version, CI on main, licence, each with its date; snapshot
   date. "No data" when missing.
5. Footer: maintained date, deploy sha, repo link, hamardikan.com.

Not on the page: real SSH/SSM smoke results (unresolved), the CV "7.4 → 2.6 min" figure, stars
(0), open issues, peer stats.

## Claims table

Status: OK = checked against the source (evidence in
`~/workspace/sandbox/showcase-scan/build/lazyss/capture/EVIDENCE.md`), Off = left off the page.

| # | Page text (draft) | Source | Reader check | Status |
|---|---|---|---|---|
| 1 | A terminal cockpit for your SSH and SSM hosts. | README 14–18; formula `desc` "Terminal machine cockpit for SSH and AWS SSM" | Read README / `Formula/lazyss.rb` | OK |
| 2 | `brew install --formula hmrdkn-labs/tap/lazyss` | `gh api repos/hmrdkn-labs/homebrew-tap` (public); formula version 0.2.0, checked 2026-11-25 | Run the command | OK |
| 3 | Latest release v0.2.0, 13 July 2026 | `gh release list --repo hmrdkn-labs/lazyss` | Release page | OK (snapshot) |
| 4 | MIT licence | `gh api repos/hmrdkn-labs/lazyss --jq .license.spdx_id` | LICENSE | OK (snapshot) |
| 5 | CI on main: success, 13 July 2026 | `gh run list --branch main --event push --limit 1` | Actions tab | OK (snapshot) |
| 6 | `g` checks the selected host; `G` checks every visible host, rows update as results arrive. "At most eight checks run at once, and each TCP connection attempt gives up after three seconds." | README 85; CHANGELOG 28; `cmd/lazyss/main.go:166` (limit 8); `sshconfig.go:298-306` (3 s dial); `internal/app/app.go:195` (stream). No overall sweep deadline and `ssh -G` has no timeout, so the page does not say "bounded" | The recorded sweep on the page | OK (EVIDENCE 1) |
| 7 | SSH check: `ssh -G` then one TCP connect, 3 s timeout, no login. SSM check: the agent ping status from the inventory load, no new call | `internal/adapters/sshconfig/sshconfig.go:228-314`; `internal/adapters/awsssm/awsssm.go:325-341` | Read the adapters | OK (EVIDENCE 2) |
| 8 | "lazyss does not read or keep" your AWS credentials; the AWS SDK finds them the standard way. The state file holds profile and region, pins, notes, tags and check and session history; never AWS credentials, the SSO cache or private keys | README 39–40, 78–79; source search, no `os.Getenv`; `awsssm.go:55-72`; `statejson/state.go`, `domain.go:91-103` (fields) | Search the source | OK (EVIDENCE 3) |
| 9 | Cleanup is a dry run; `--check` also tries a TCP connection to each host; `--write` with `--host` saves `<config>.lazyss-backup-<timestamp>` first, writes only duplicate targets and refuses protected identities such as github.com | README 91–98; `cleanup.go:128`; synthetic run: `cmp` unchanged, `--check` dialled hosts, protected write exit 1 | Run on a copy | OK (EVIDENCE 4) |
| 10 | Release archives ship with `checksums.txt` | README 183–185; release assets | Release page | OK |
| 11 | Capture caption: date, version, 10 hosts, "synthetic SSH config", SSH only | `data/sweep.json` | The JSON in the repo | OK |
| 12 | "Snapshot taken <date>. The build reads GitHub once; this page never calls it." | `site/scripts/build.sh` (gh calls at build); `public/sweep.js` fetches only `/data/replay.json`; CSP `connect-src 'self'` in `_headers` | Read the build script, `sweep.js` and `_headers` | OK |
| 13 | "The Homebrew formula is the primary install path on macOS and Linux." | README 195; formula `on_macos`/`on_linux` blocks | Read the formula | OK |

## Real vs missing

- Real: the sweep capture, release and CI snapshot, formula version, README behaviour verified in
  code.
- Missing (for Dika): smoke status wording; a recorded clean install from the tap; SSM rows in the
  capture (would need a real AWS account, so the capture is SSH-only and says so).

## Self-critique

"Would a design-literate engineer guess this was AI-generated? Why not?"

Mostly no. The centre of the page is not an illustration but the program's own output, printed in
two ribbon inks on paper colour, with the tractor strip as the only ornament. Section heads are
listing line numbers (10, 20, …) in the margin, not cards; rules are hairlines; there is one
button style, square, ruled. The replay uses the recorded timings, so the two timeouts land late
(3.3 s) the way they did on the Mac, which a generated mock would not bother to do.

What still smelled after round 1, and what was done:
- At 375 px the Health column sat off-screen behind a 115 px gutter. Gutter cut to a 12 px strip,
  and under 1280 px a plain list "In the order they came back" (time, host, result) sits under the
  capture, so the result is readable without scrolling the box.
- Hover/tap notes on host lines were 20 px targets and disagreed with the replayed frame (review
  round 1). Host lines are no longer controls; result times print as static margin annotations.
- "A sweep is bounded" implied an overall deadline that does not exist. Now names the two real
  limits.
- An Oxford comma in "What it reads". Rewritten.
- The 100-column run's first frame landed at +0.3 s with eight results at once, so the margin reads
  "+0.3 s" eight times. Kept: it is what the recording shows, and the caption says the times are
  when results showed on screen.
- Remaining risk: the run of short dry sentences in sections 20–50 reads as a house rhythm.
  Kept, because every sentence there carries a checked fact.

## PLAYBOOK §8 checklist

Evidence: `site/scripts/verify.mjs` run 2026-11-25 and shots in
`~/workspace/sandbox/showcase-scan/build/lazyss/shots/`.

Copy
- [x] First line says what and for whom: h1 "A terminal cockpit for your SSH and SSM hosts."
- [x] Meta and og description name the project in one line (`index.html` head).
- [x] One role list / contact order: n/a (project page; footer links repo and hamardikan.com only).
- [x] Team size and denominators: n/a (one maintainer; host count "10" carries its denominator).
- [x] No ⚠️, [confirm], TBD, XX: `rg -n '⚠|\[confirm\]|TBD|XX' site/public` empty.
- [x] Voice: read aloud; no exclamation marks; curly quotes; no Oxford comma.
- [x] Claims table all OK (above).

Claims and data
- [x] Numbers carry unit and window: "10 hosts", "3.3 s", "eight checks", "three seconds"; signals
  carry dates.
- [x] Live numbers dated: each signal has "published"/"checked" date and a snapshot date.
- [x] No placeholders or empty sections.

Accessibility and phone
- [x] Skip link first (keyboard shot), one h1, `lang="en"` (verify.mjs asserts).
- [x] Contrast AA light and dark: axe zero violations in both schemes.
- [x] Zero console errors, no failed requests (verify.mjs).
- [x] No horizontal scroll at 320, 375, 820 (verify.mjs `scrollWidth <= innerWidth`).
- [x] Touch copy: no hover-only or keyboard-only instruction on the page; the 60-column box says it
  scrolls sideways, and the result list under it needs no gesture (375 shot).
- [x] Things a thumb can press before first scroll at 375: skip link (hidden until focus), Copy,
  "Release notes for v0.2.0". Three.
- [x] Reduced motion: finished frame, no replay control (shot). No loop; the replay runs once,
  3.3 s plus a 0.7 s lead, under the 5 s pause threshold.
- [x] JS off: page complete, finished frame pre-rendered (`1440-js-off.png`).

Visuals
- [x] One signature object from a real artifact, static fallback in HTML.
- [x] Nothing from the banned list: no gradients, glow, cards, chips, window chrome, cursor, emoji;
  fonts IBM Plex Mono and Source Serif 4.
- [x] Self-critique answered (above).

Privacy
- [x] No IPs, ports, internal hostnames, account IDs or paths: `rg` over `site/public` and
  `site/data` finds none (capture EVIDENCE leak check; verify.mjs grep).
- [x] No "ADI", no CV link, no contribution splits.

Deploy
- [x] Root-absolute URLs for internal links and assets.
- [x] No `_redirects`.
- [x] CSP allows only `'self'` (`_headers`); no console CSP errors live. The zone's Web Analytics
  beacon was injected into HTML and blocked by the CSP; `Cache-Control: no-transform` on `/*` stops
  the injection (`~/universe/kb/cloudflare.md`).
- [x] `/assets/*` immutable; font files carry weight in the name, rename on change.
- [x] No layout shift: capture height reserved by fixed rows in CSS; fonts preloaded.
- [x] Live copy checked after deploy: live `index.html` byte-identical to `site/public/index.html`;
  `BASE_URL=https://lazyss.hamardikan.com node scripts/verify.mjs` passes (axe zero, no console
  errors, no overflow, 122.9 KB at 375); shots in `shots/live/`.

Lighthouse mobile, live, three runs on 2026-11-25: performance 100, accessibility 100, best
practices 100, SEO 100 each.

Review: three rounds on `openai-codex/gpt-6.1-sol` (`build/lazyss/REVIEW.md`); round 3 open
must-fix 0.
