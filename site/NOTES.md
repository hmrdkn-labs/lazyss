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

- Desktop: the 100-column capture. Phone (<700 px): the 60-column capture in a box that scrolls
  sideways with a visible scrollbar; text is never scaled down.
- **Signature motion:** "print". When the reader presses "Run the sweep again" (and once on first view
  with motion allowed), frames advance at their real `t_ms` offsets; a line that changes gets a
  short ink-darken (opacity 0.55 → 1, 180 ms), like a fresh strike of the ribbon. No typing cursor.
  The whole run is under 10 s and does not loop.
- **Static fallback:** the finished frame is pre-rendered into `index.html` at build time. With
  JavaScript off or `prefers-reduced-motion: reduce`, that is what shows; the replay button is not
  rendered.
- **Micro-interactions (3):** (1) focus or hover a host line: the margin shows when its result came
  back ("+1.2 s"), from the frame data; (2) "Copy" beside the brew command says "Copied" for 2 s,
  with a live region; (3) the replay control shows elapsed time against the real run length.

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
| 6 | `g` checks the selected host; `G` checks every visible host, up to eight at once, rows update as results arrive | README 85; CHANGELOG 28; `cmd/lazyss/main.go:166` (limit 8); `internal/app/app.go:195` (stream) | The recorded sweep on the page | OK (EVIDENCE 1) |
| 7 | SSH check: `ssh -G` then one TCP connect, 3 s timeout, no login. SSM check: the agent ping status from the inventory load, no new call | `internal/adapters/sshconfig/sshconfig.go:228-314`; `internal/adapters/awsssm/awsssm.go:325-341` | Read the adapters | OK (EVIDENCE 2) |
| 8 | Stores profile and region labels only; lazyss never reads or stores AWS credentials or the SSO cache (the AWS SDK signs calls with your usual chain when an AWS source is on) | README 39–40, 78–79; source search, no `os.Getenv`; `awsssm.go:55-72` | Search the source | OK (EVIDENCE 3) |
| 9 | Cleanup is a dry run; `--write` needs named hosts and writes `config.lazyss-backup-<timestamp>` first; SCM identity hosts refuse writes | README 91–98; `cleanup.go:128`; synthetic run, `cmp` unchanged | Run on a copy | OK (EVIDENCE 4) |
| 10 | Release archives ship with `checksums.txt` | README 183–185; release assets | Release page | OK |
| 11 | Capture caption: date, version, 10 hosts, "synthetic SSH config", SSH only | `data/sweep.json` | The JSON in the repo | OK |

## Real vs missing

- Real: the sweep capture, release and CI snapshot, formula version, README behaviour verified in
  code.
- Missing (for Dika): smoke status wording; a recorded clean install from the tap; SSM rows in the
  capture (would need a real AWS account, so the capture is SSH-only and says so).

## Self-critique

(Filled after the first full set of shots.)

## PLAYBOOK §8 checklist

(Filled with evidence before review.)
