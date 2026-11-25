# LazySS landing page: notes

Live: https://lazyss.hamardikan.com/ · Source: `site/` · Worker: `lazyss-site`. This is v2; v1 (paper, tractor strip, serif, teletype) was thrown away on 2026-11-25 and only its verified facts and plumbing were kept.

## Concept

**The operations room in daylight.** lazyss lives in a dark terminal pane and reaches machines in racks. Its
own colours (from the real capture) are navy `#0b1220`, sky `#7dd3fc`, green up, red down. The page is a
cool stone wall with one dark screen in it: colour-blocking, not a fake terminal. Spec: `build/v2/lazyss/DIRECTION.md`.

- **Palette** (`:root`): `--bg` `#eef2f4`, `--surface` `#dde5ea`, `--text` `#0b1220`, `--muted` `#46525e`,
  `--accent` `#0b5e7e` (the one button, links, focus ring), `--stage` `#0b1220` (recording band only). No
  beige, purple, gradient, shadow.
- **Type:** Mona Sans (GitHub, OFL) for the h1 (wide, 800) and all prose; IBM Plex Mono (OFL) for the
  recording, keys and the brew command only. Sizes below the hero: 2.5rem (h2), 1.125rem (prose, h3-as-bold,
  buttons), 0.875rem (recording, captions, footer). Nothing under 14 px.
- **Hero:** centered monumental headline across about 92% of the width, one 12-word line, one button, the navy
  stage's top edge in the first 900 px.
- **Stage:** the real recording (`data/sweep.json`), 100 columns from 1280 px, 60 below (sideways scroll, text
  never scaled), replayed once on view at real `t_ms`, then "Run the sweep again" with elapsed time. Static
  fallback: the finished frame in the HTML (JS off, reduced motion: no replay control).
- **Images:** three `generate_image` illustrations, one look (matte studio photograph, north daylight, pale
  stone ground, navy and brushed steel, one teal accent, no text, no screens). They are illustration: alt text
  says so and none is presented as a capture. Prompts are in the worklog (`build/v2/lazyss/WORKLOG-w1.md`).
- **Colour fix:** the capture's dim grey `#586069` is 2.9:1 on navy, so dim text prints as the capture's other
  grey `#8b949e` (6.1:1). Text and structure are as recorded.

## Page outline

1. Header: wordmark, three links (How it works, Install, GitHub), one line.
2. Hero (h1, one line, "Install with Homebrew") then the stage with caption.
3. See every host’s reach at once (text first, then a full-bleed 3.6:1 band);  Your credentials stay where they are; Cleanup shows you the change first.
4. Install: the brew command, Copy, release archives and `checksums.txt`.
5. Where the project stands: latest release, formula, CI on main, licence, each dated; snapshot date.
6. Footer: deploy sha, licence, repo, hamardikan.com.

Not on the page: real SSH/SSM smoke results (unresolved), the CV "7.4 → 2.6 min" figure, stars (0), open
issues, peer stats, and the per-host result times of v1 (the replay still runs at those times).

## Claims table

Status: OK = checked against the source (evidence in `~/workspace/sandbox/showcase-scan/build/lazyss/capture/EVIDENCE.md`), Off = left off the page. Rows 1-13 are the v1 rows, reused; v2 wording of each sentence was re-read against its row on 2026-11-25 (the page no longer says the connection is closed at once).

| # | Page text (draft) | Source | Reader check | Status |
|---|---|---|---|---|
| 1 | A terminal cockpit for your SSH and SSM hosts. | README 14–18; formula `desc` "Terminal machine cockpit for SSH and AWS SSM" | Read README / `Formula/lazyss.rb` | OK |
| 2 | `brew install --formula hmrdkn-labs/tap/lazyss` | `gh api repos/hmrdkn-labs/homebrew-tap` (public); formula version 0.2.0, checked 2026-11-25 | Run the command | OK |
| 3 | Latest release v0.2.0, 13 July 2026 | `gh release list --repo hmrdkn-labs/lazyss` | Release page | OK (snapshot) |
| 4 | MIT licence | `gh api repos/hmrdkn-labs/lazyss --jq .license.spdx_id` | LICENSE | OK (snapshot) |
| 5 | CI on main: success, 25 November 2026 (run after the Go 1.26.11 bump, PR #58; before that the last push run was 13 July 2026) | `gh run list --branch main --event push --limit 1` | Actions tab | OK (snapshot) |
| 6 | `g` checks the selected host; `G` checks every visible host, rows update as results arrive. "At most eight checks run at once, and each TCP connection attempt gives up after three seconds." | README 85; CHANGELOG 28; `cmd/lazyss/main.go:166` (limit 8); `sshconfig.go:298-306` (3 s dial); `internal/app/app.go:195` (stream). No overall sweep deadline and `ssh -G` has no timeout, so the page does not say "bounded" | The recorded sweep on the page | OK (EVIDENCE 1) |
| 7 | SSH check: `ssh -G` then one TCP connect, 3 s timeout, no login. SSM check: the agent ping status from the inventory load, no new call | `internal/adapters/sshconfig/sshconfig.go:228-314`; `internal/adapters/awsssm/awsssm.go:325-341` | Read the adapters | OK (EVIDENCE 2) |
| 8 | "lazyss does not read or keep" your AWS credentials; the AWS SDK finds them the standard way. The state file holds profile and region, pins, notes, tags and check and session history; never AWS credentials, the SSO cache or private keys | README 39–40, 78–79; source search, no `os.Getenv`; `awsssm.go:55-72`; `statejson/state.go`, `domain.go:91-103` (fields) | Search the source | OK (EVIDENCE 3) |
| 9 | Cleanup is a dry run; `--check` also tries a TCP connection to each host; `--write` with `--host` saves `<config>.lazyss-backup-<timestamp>` first, writes only duplicate targets and refuses protected identities such as github.com | README 91–98; `cleanup.go:128`; synthetic run: `cmp` unchanged, `--check` dialled hosts, protected write exit 1 | Run on a copy | OK (EVIDENCE 4) |
| 10 | Release archives ship with `checksums.txt` | README 183–185; release assets | Release page | OK |
| 11 | Capture caption: date, version, 10 hosts, "synthetic SSH config", SSH only, run length; "A recording, not a live feed" | `data/sweep.json` | The JSON in the repo | OK |
| 12 | "Snapshot taken <date>. The build reads GitHub once; this page never calls it." | `site/scripts/build.sh` (gh calls at build); `public/sweep.js` fetches only `/data/replay.json`; CSP `connect-src 'self'` in `_headers` | Read the build script, `sweep.js` and `_headers` | OK |
| 13 | "The Homebrew formula is the primary install path on macOS and Linux." | README 195; formula `on_macos`/`on_linux` blocks | Read the formula | OK |

## Real vs missing

- Real: the sweep capture, release and CI snapshot, formula version, README behaviour verified in
  code.
- Missing (for Dika): smoke status wording; a recorded clean install from the tap; SSM rows in the
  capture (would need a real AWS account, so the capture is SSH-only and says so).

## Self-critique

"Would a design-literate engineer guess this was AI-generated? Why not?"

Less likely than v1, not impossible. The type does the work: Mona Sans at wide 800 across the full width,
then a navy screen that is the program's own recording, not an illustration of one. No cards, chips,
eyebrows, numbering, stat strip or gradients; one button style. The replay keeps the recorded timings, so the
two timeouts land late (3.3 s) as they did on the Mac.

Remaining risks:
- Alternating text and image splits are the standard marketing rhythm. The images are specific objects
  (ten blocks, a lockbox, a sealed stack), but the layout is conventional.
- The generated images are glossy renders; they sit next to a real capture, and the alt text says they
  are illustrations. The reach image was cropped to its subject to avoid empty stone.
- The signals band has empty space to the right of the list on desktop. Kept: a list of four dated facts
  does not need a second column.
- The 60-column recording on a phone scrolls sideways; the first screen shows the Method column cut off.
  Kept: text is never scaled, and the box is a labelled, focusable scroll region.

## Checks (v2)

`node scripts/audit.mjs` on 2026-11-25: two font families, three sizes below the hero (40, 18, 14 px), min 14 px,
no uppercase with tracking, no `transition: all`, no em or spaced en dashes, one h1 at 1325 px of 1440,
no horizontal overflow at 320/390/1440, axe zero at 1440 and 390, 153 KB at 390 (replay excluded), JS-off
page complete, no console errors, Lighthouse mobile as printed by the script. Shots:
`~/workspace/sandbox/showcase-scan/build/v2/lazyss/shots/w1/`. Privacy grep from `README.md`: empty.
The live check, the reviewer rounds and the parent gate are still to do.
