// v2 audit of the landing page in Chromium. Prints ok/FAIL per check and exits 1 on any FAIL.
//   node scripts/audit.mjs              serves public/ on a free local port
//   BASE_URL=https://lazyss.hamardikan.com node scripts/audit.mjs   audits a live site
//   NO_LH=1 node scripts/audit.mjs      skips Lighthouse (mobile: performance >= 90, accessibility = 100)
// Hero = [data-hero], else the h1's closest section. Playwright, axe and lighthouse live in ~/.cache/lazyss-pw:
//   npm i --prefix ~/.cache/lazyss-pw playwright @axe-core/playwright lighthouse && npx --prefix ~/.cache/lazyss-pw playwright install chromium
import { createRequire } from 'node:module';
import { homedir } from 'node:os';
import { spawn, spawnSync } from 'node:child_process';
import { createServer } from 'node:net';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(`${homedir()}/.cache/lazyss-pw/`);
const { chromium } = require('playwright');
const AxeBuilder = require('@axe-core/playwright').default;

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const freePort = () => new Promise((res, rej) => {
  const s = createServer().once('error', rej).listen(0, '127.0.0.1', () => { const { port } = s.address(); s.close(() => res(port)); });
});
let server = null;
let base = process.env.BASE_URL?.replace(/\/$/, '');
if (!base) {
  const port = await freePort();
  base = `http://127.0.0.1:${port}`;
  server = spawn('python3', ['-m', 'http.server', String(port), '-d', `${root}/public`, '-b', '127.0.0.1'], { stdio: 'ignore' });
  await new Promise((r) => setTimeout(r, 800));
}

const failures = [];
const check = (ok, msg) => { console.log(`${ok ? 'ok  ' : 'FAIL'} ${msg}`); if (!ok) failures.push(msg); };
const problems = [];
const browser = await chromium.launch();

function watch(page, label) {
  page.on('console', (m) => { if (m.type() === 'error') problems.push(`${label}: console error: ${m.text()}`); });
  page.on('pageerror', (e) => problems.push(`${label}: page error: ${e.message}`));
  page.on('requestfailed', (r) => problems.push(`${label}: request failed: ${r.url()}`));
  page.on('response', (r) => { if (r.status() >= 400) problems.push(`${label}: ${r.status()} ${r.url()}`); });
}
async function open(opts, label) {
  const ctx = await browser.newContext(opts);
  const page = await ctx.newPage();
  watch(page, label);
  await page.goto(base + '/', { waitUntil: 'networkidle' });
  await page.evaluate(() => document.fonts.ready);
  return { ctx, page };
}

// runs in the page: every element with its own visible text, with computed type properties
function collect() {
  const hero = document.querySelector('[data-hero]') ?? document.querySelector('h1')?.closest('section') ?? document.querySelector('h1')?.parentElement;
  const sel = (e) => {
    let s = e.tagName.toLowerCase();
    if (e.id) return `${s}#${e.id}`;
    if (typeof e.className === 'string' && e.className.trim()) s += '.' + e.className.trim().split(/\s+/).join('.');
    const p = e.parentElement;
    return p && p !== document.body ? `${p.tagName.toLowerCase()} > ${s}` : s;
  };
  const out = [];
  for (const e of document.body.querySelectorAll('*')) {
    if (['SCRIPT', 'STYLE', 'NOSCRIPT', 'TEMPLATE'].includes(e.tagName)) continue;
    if (![...e.childNodes].some((n) => n.nodeType === 3 && n.textContent.trim())) continue;
    const cs = getComputedStyle(e);
    const r = e.getBoundingClientRect();
    if (cs.display === 'none' || cs.visibility === 'hidden' || r.width === 0 || r.height === 0) continue;
    out.push({
      sel: sel(e),
      family: cs.fontFamily.split(',')[0].trim().replace(/^["']|["']$/g, ''),
      size: parseFloat(cs.fontSize),
      below: !e.closest('header') && !(hero && hero.contains(e)),
      upper: cs.textTransform === 'uppercase' && parseFloat(cs.letterSpacing) > 0,
    });
  }
  const trans = [];
  for (const e of document.querySelectorAll('*')) {
    if (getComputedStyle(e).transitionProperty.split(',').some((p) => p.trim() === 'all')) {
      // transition-property defaults to "all" with duration 0s; only a running duration counts
      if (getComputedStyle(e).transitionDuration.split(',').some((d) => parseFloat(d) > 0)) trans.push(sel(e));
    }
  }
  return { out, trans, hasHero: !!hero };
}

const group = (items, key) => {
  const m = new Map();
  for (const i of items) { const k = key(i); if (!m.has(k)) m.set(k, []); m.get(k).push(i.sel); }
  return m;
};
const fmt = (m) => [...m].map(([k, v]) => `${k} (${v[0]}${v.length > 1 ? ` +${v.length - 1}` : ''})`).join(', ');

try {
  // style and copy checks at 1440, motion allowed
  {
    const { ctx, page } = await open({ viewport: { width: 1440, height: 900 } }, '1440');
    const { out, trans, hasHero } = await page.evaluate(collect);
    const fams = group(out, (i) => i.family);
    check(fams.size <= 2, `font families ${fams.size} <= 2: ${fmt(fams)}`);
    check(hasHero, 'hero found ([data-hero] or the h1 section)');
    const sizes = group(out.filter((i) => i.below), (i) => `${i.size}px`);
    check(sizes.size <= 3, `font sizes below the hero ${sizes.size} <= 3: ${fmt(sizes)}`);
    const small = out.reduce((a, b) => (b.size < a.size ? b : a), out[0] ?? { size: Infinity, sel: 'none' });
    check(small.size >= 13, `min font size ${small.size}px >= 13 px (${small.sel})`);
    const upper = out.filter((i) => i.upper);
    check(upper.length === 0, `uppercase with letter-spacing: ${upper.length}${upper.length ? ' e.g. ' + upper.slice(0, 3).map((i) => i.sel).join(', ') : ''}`);
    check(trans.length === 0, `transition-property all: ${trans.length}${trans.length ? ' e.g. ' + trans.slice(0, 3).join(', ') : ''}`);
    const text = await page.evaluate(() => ({
      body: document.body.innerText,
      meta: [document.title, ...[...document.querySelectorAll('meta[name=description],meta[property="og:description"],meta[name="twitter:description"]')].map((m) => m.content)],
    }));
    const bad = [...text.body.split('\n'), ...text.meta].filter((l) => /—| – /.test(l));
    check(bad.length === 0, `em or spaced en dashes: ${bad.length}${bad.length ? ' e.g. "' + bad[0].slice(0, 60) + '"' : ''}`);
    const h1 = await page.locator('h1').evaluateAll((n) => n.map((e) => e.getBoundingClientRect().width));
    check(h1.length === 1, `${h1.length} h1`);
    check(h1.length === 1 && h1[0] >= 0.7 * 1440, `h1 width ${h1[0]?.toFixed(0)} px >= ${0.7 * 1440 | 0} px (70% of 1440)`);
    const res = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa', 'wcag22aa']).analyze();
    check(res.violations.length === 0, `axe 1440: ${res.violations.length} violations ${res.violations.map((v) => `${v.id}(${v.nodes.length})`).join(' ')}`);
    await ctx.close();
  }

  // phone: overflow, axe, transfer size
  {
    const ctx = await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true });
    const page = await ctx.newPage();
    watch(page, '390');
    let bytes = 0;
    const pending = [];
    page.on('response', (r) => { if (!/replay\.json(\?|$)/.test(r.url())) pending.push(r.body().then((b) => { bytes += b.length; }, () => {})); });
    await page.goto(base + '/', { waitUntil: 'networkidle' });
    await page.evaluate(() => document.fonts.ready);
    await page.waitForTimeout(500);
    await Promise.all(pending);
    check(bytes < 300 * 1024, `390 transfer ${(bytes / 1024).toFixed(1)} KB < 300 KB (replay.json excluded, uncompressed)`);
    const res = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa', 'wcag22aa']).analyze();
    check(res.violations.length === 0, `axe 390: ${res.violations.length} violations ${res.violations.map((v) => `${v.id}(${v.nodes.length})`).join(' ')}`);
    await ctx.close();
  }
  for (const w of [320, 390, 1440]) {
    const { ctx, page } = await open({ viewport: { width: w, height: 800 } }, `w${w}`);
    const [sw, iw] = await page.evaluate(() => [document.documentElement.scrollWidth, innerWidth]);
    check(sw <= iw, `scrollWidth ${sw} <= innerWidth ${iw} at ${w}`);
    await ctx.close();
  }

  // JS off
  {
    const { ctx, page } = await open({ viewport: { width: 1440, height: 900 }, javaScriptEnabled: false }, 'nojs');
    const r = await page.evaluate(() => {
      const h1 = document.querySelector('h1');
      const rec = document.querySelector('[data-recording], [data-replay], .recording, .scroll-100, .cap, pre');
      return { h1: !!h1 && h1.innerText.trim().length > 0, rec: !!rec && rec.innerText.trim().length > 0 };
    });
    check(r.h1 && r.rec, `JS off: h1 ${r.h1 ? 'present' : 'MISSING'}, pre-rendered recording ${r.rec ? 'present' : 'MISSING'}`);
    await ctx.close();
  }

  check(problems.length === 0, `console errors and failed requests: ${problems.length}${problems.length ? '\n       ' + [...new Set(problems)].slice(0, 8).join('\n       ') : ''}`);
} finally {
  await browser.close();
}

// Lighthouse mobile
if (process.env.NO_LH) {
  console.log('skip Lighthouse (NO_LH=1)');
} else {
  const pw = `${homedir()}/.cache/lazyss-pw`;
  const bin = `${pw}/node_modules/.bin/lighthouse`;
  const probe = spawnSync('test', ['-x', bin]);
  if (probe.status !== 0) spawnSync('npm', ['i', '--prefix', pw, 'lighthouse'], { stdio: 'inherit' });
  const exe = chromium.executablePath();
  const lh = spawnSync('npx', ['--prefix', pw, 'lighthouse', `${base}/`, '--chrome-flags=--headless=new', '--only-categories=performance,accessibility', '--output=json', '--output-path=stdout', '--quiet'], {
    env: { ...process.env, CHROME_PATH: exe }, encoding: 'utf8', maxBuffer: 64 * 1024 * 1024,
  });
  try {
    const j = JSON.parse(lh.stdout);
    const perf = Math.round(j.categories.performance.score * 100);
    const a11y = Math.round(j.categories.accessibility.score * 100);
    check(perf >= 90, `Lighthouse mobile performance ${perf} >= 90`);
    check(a11y === 100, `Lighthouse mobile accessibility ${a11y} = 100`);
  } catch {
    check(false, `Lighthouse did not produce a report: ${(lh.stderr || '').slice(0, 300)}`);
  }
}

server?.kill();
console.log(failures.length ? `\n${failures.length} FAILED` : '\nall checks passed');
process.exit(failures.length ? 1 : 0);
