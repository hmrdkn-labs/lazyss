// Pre-renders data/sweep.json and public/data/signals.json into public/index.html, between marker comments,
// so the page is complete with JavaScript off. Also writes public/data/replay.json, the frames sweep.js replays.
// Usage: node scripts/render.mjs [path/to/sweep.json]   (default: data/sweep.json; env SWEEP_JSON overrides)
import { readFileSync, writeFileSync, renameSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const sweepPath = process.argv[2] ?? process.env.SWEEP_JSON ?? `${root}/data/sweep.json`;
const sweep = JSON.parse(readFileSync(sweepPath, 'utf8'));
const signals = JSON.parse(readFileSync(`${root}/public/data/signals.json`, 'utf8'));

const esc = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
const MONTHS = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];
const day = (iso) => {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? null : `${d.getUTCDate()} ${MONTHS[d.getUTCMonth()]} ${d.getUTCFullYear()}`;
};

// Terminal colours become the page palette by hue, whatever palette the terminal used. Green (healthy) prints as
// plain ink: the ribbon is red for exceptions only. Status stays a word as well as a colour.
function fgClass(hex) {
  const m = /^#([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$/i.exec(hex ?? '');
  if (!m) return '';
  const [r, g, b] = [m[1], m[2], m[3]].map((x) => parseInt(x, 16));
  const lum = (r + g + b) / 3;
  if (Math.max(r, g, b) - Math.min(r, g, b) < 28) return lum < 0xd0 && lum > 0x30 ? 'dim' : '';
  if (r >= g + 50 && r >= b + 50) return g > 0xa0 ? 'warn' : 'bad';
  if (g >= r + 30 && g >= b + 30) return 'ok';
  if (b >= r + 30) return 'acc';
  return '';
}
// Plex Mono has no glyph for these: each is boxed to exactly one cell so the columns keep their grid.
const CELL = /[●◉◌○▸▶◆◇■□▲▼]/g;

function spanHtml(s) {
  const cls = [];
  if (s.b) cls.push('b');
  if (s.dim) cls.push('dim');
  const fg = fgClass(s.fg);
  if (fg) cls.push(`c-${fg}`);
  if (s.bg || s.rev) cls.push('hl');
  const text = esc(s.t).replace(CELL, (g) => `<span class="g">${g}</span>`);
  return cls.length ? `<span class="${cls.join(' ')}">${text}</span>` : text;
}
const lineHtml = (spans) => spans.map(spanHtml).join('');
const lineText = (spans) => spans.map((s) => s.t).join('');

// A host line is a list row: "│›   ssh api     ssh     ● up   never │" (marker, source, name, method, health).
const HOST = /^│?[›\s]\s*(?:ssh|ssm)\s+(\S+)\s+(?:ssh|ssm)\s+(\S.*?)(?:\s{2,}.*)?\s*│?$/;

function size(key) {
  const frames = sweep.sizes[key].frames;
  const rows = frames.map((f) => f.lines.map(lineHtml));
  const texts = frames.map((f) => f.lines.map(lineText));
  const last = frames.length - 1;
  const hosts = new Map();
  texts[last].forEach((t, i) => {
    const m = HOST.exec(t);
    if (!m) return;
    let k = last;
    while (k > 0 && texts[k - 1][i] === t) k--;
    hosts.set(i, { name: m[1], health: m[2].replace(/^[^\p{L}]+/u, ''), t_ms: k === 0 ? null : frames[k].t_ms });
  });
  return { frames, rows, last, hosts, cols: sweep.sizes[key].cols };
}

const sizes = { 100: size('100'), 60: size('60') };
const hostCount = sizes[100].hosts.size;
for (const k of ['100', '60']) if (sizes[k].hosts.size !== hostCount) throw new Error(`size ${k} shows ${sizes[k].hosts.size} hosts, 100 shows ${hostCount}`);
if (hostCount === 0) throw new Error('no host lines found in the final frame');
const unfinished = [...sizes[100].hosts.values()].filter((h) => /check|unchecked/i.test(h.health));
if (unfinished.length) throw new Error(`final frame still checking: ${unfinished.map((h) => h.name).join(', ')}`);

const secs = (ms) => `+${(ms / 1000).toFixed(1)} s`;

function figure(key) {
  const s = sizes[key];
  const lines = s.rows[s.last].map((html, i) => {
    const h = s.hosts.get(i);
    if (!h) return `<div class="ln" data-i="${i}"><span class="tx">${html}</span></div>`;
    const note = h.t_ms == null ? '' : `<span class="note" aria-hidden="true" data-ms="${h.t_ms}">${secs(h.t_ms)}</span>`;
    return `<div class="ln host" data-i="${i}">${note}<span class="tx">${html}</span></div>`;
  });
  return `<div class="scroll scroll-${key}" tabindex="0" role="region" aria-label="Recorded sweep, ${key} columns, scrolls sideways">
<div class="paper"><span class="strip" aria-hidden="true"></span>
<div class="cap" data-cols="${s.cols}" data-rows="${s.rows[0].length}">
${lines.join('')}
</div></div></div>`;
}

// under 1280 px: the hosts in the order their results came back (times from the 60-column recording it sits under)
function order() {
  const s = sizes[60];
  const bad = (i) => sweep.sizes['60'].frames[s.last].lines[i].some((x) => fgClass(x.fg) === 'bad');
  const items = [...s.hosts.entries()]
    .map(([i, h]) => ({ i, ...h }))
    .sort((a, b) => (a.t_ms ?? 0) - (b.t_ms ?? 0) || a.i - b.i)
    .map((h) => `<li data-ms="${h.t_ms ?? 0}"><span class="t">${secs(h.t_ms ?? 0)}</span><span class="a">${esc(h.name)}</span><span class="r${bad(h.i) ? ' c-bad' : ''}">${esc(h.health)}</span></li>`);
  return `<div class="order"><h3 class="label">In the order they came back</h3>
<ol>
${items.join('\n')}
</ol></div>`;
}

const run = { 100: sizes[100].frames[sizes[100].last].t_ms, 60: sizes[60].frames[sizes[60].last].t_ms };
const version = (/\d+\.\d+\.\d+/.exec(sweep.lazyss_version) ?? [])[0];
if (!version) throw new Error(`cannot read a version from ${JSON.stringify(sweep.lazyss_version)}`);
if (!/synthetic/i.test(sweep.inventory)) throw new Error('sweep.json inventory does not say synthetic');
const finalHosts = [...sizes[100].hosts.values()];
const results = finalHosts.map((h) => `${h.name}, ${h.health}`).join('; ');

const sweepBlock = `${figure('100')}
${figure('60')}
${order()}
<p class="vh">Final frame of the recorded sweep, one result per host: ${esc(results)}.</p>
<figcaption>Recording of lazyss ${version}, captured ${day(sweep.captured_at)}: ${hostCount} hosts from a synthetic SSH config, SSH checks only. The results are real TCP checks against throwaway listeners, closed ports and unreachable test addresses, and the sweep took <span class="w100">${(run[100] / 1000).toFixed(1)}</span><span class="w60">${(run[60] / 1000).toFixed(1)}</span> s. The times beside each host are when its result first showed on screen in the recording, not the check’s own latency. A recording, not a live feed. Terminal colours are redrawn in the page’s two inks; the text is as the terminal printed it.</figcaption>`;

// the frames sweep.js replays: the first in full, then only the lines that changed
const replay = {};
for (const k of ['100', '60']) {
  const s = sizes[k];
  const frames = s.rows.map((rows, n) => {
    const c = {};
    rows.forEach((h, i) => { if (n === 0 || h !== s.rows[n - 1][i]) c[i] = h; });
    return { t: s.frames[n].t_ms, c };
  });
  replay[k] = { rows: s.rows[0].length, run_ms: run[k], frames };
}

const dl = (rows) => rows.map(([k, v, d]) => `<div><dt>${k}</dt><dd>${v}${d ? ` <span class="when">${d}</span>` : ''}</dd></div>`).join('\n');
const ND = 'No data';
const snap = day(signals.generated_at) ?? ND;
const r = signals.release, f = signals.formula, c = signals.ci, l = signals.license;
const signalsBlock = `<dl class="signals">
${dl([
  ['Latest release', r ? `<a href="${esc(r.url)}">${esc(r.tag)}</a>` : ND, r ? `published ${day(r.published_at)}` : ''],
  ['Formula in the tap', f ? esc(f) : ND, f ? `checked ${snap}` : ''],
  ['CI on main', c ? `<a href="${esc(c.url)}">${esc(c.conclusion)}</a>` : ND, c ? `last push run ${day(c.created_at)}` : ''],
  ['Licence', l ? esc(l) : ND, l ? `checked ${snap}` : ''],
])}
</dl>
<p class="snapshot">Snapshot taken ${snap}. The build reads GitHub once; this page never calls it.</p>`;

const sha = execFileSync('git', ['-C', root, 'rev-parse', '--short', 'HEAD']).toString().trim();
const built = day(new Date().toISOString());
const footBlock = `<p>Maintained ${built}. Deploy <code>${sha}</code>.</p>`;

let html = readFileSync(`${root}/public/index.html`, 'utf8');
for (const [name, body] of [['sweep', sweepBlock], ['signals', signalsBlock], ['build', footBlock]]) {
  const re = new RegExp(`(<!-- ${name}:start -->)[\\s\\S]*?(<!-- ${name}:end -->)`);
  if (!re.test(html)) throw new Error(`marker ${name} missing in public/index.html`);
  html = html.replace(re, (_, a, b) => `${a}\n${body}\n${b}`);
}
const tmp = `${root}/public/.index.html.tmp`;
writeFileSync(tmp, html);
renameSync(tmp, `${root}/public/index.html`);
writeFileSync(`${root}/public/data/replay.json`, JSON.stringify(replay));
console.log(`rendered ${hostCount} hosts, run ${(run[100] / 1000).toFixed(1)} s (100) and ${(run[60] / 1000).toFixed(1)} s (60), replay.json ${JSON.stringify(replay).length} B`);
