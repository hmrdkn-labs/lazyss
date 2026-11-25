// Checks public/ in Chromium: axe (light and dark), console errors, failed requests, horizontal scroll at
// 320/375/820, one h1, phone transfer size, grid alignment of the capture. Writes shots and demo.webm.
//   node scripts/verify.mjs [shots-dir]
// Playwright lives in ~/.cache/lazyss-pw (not a dependency of this repo):
//   npm i --prefix ~/.cache/lazyss-pw playwright @axe-core/playwright && npx --prefix ~/.cache/lazyss-pw playwright install chromium
import { createRequire } from 'node:module';
import { homedir } from 'node:os';
import { spawn } from 'node:child_process';
import { mkdirSync, renameSync, readdirSync, rmSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(`${homedir()}/.cache/lazyss-pw/`);
const { chromium } = require('playwright');
const AxeBuilder = require('@axe-core/playwright').default;

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const shots = resolve(process.argv[2] ?? `${homedir()}/workspace/sandbox/showcase-scan/build/lazyss/shots`);
mkdirSync(shots, { recursive: true });
const PORT = 8793;
const base = `http://127.0.0.1:${PORT}`;
const server = spawn('python3', ['-m', 'http.server', String(PORT), '-d', `${root}/public`, '-b', '127.0.0.1'], { stdio: 'ignore' });
await new Promise((r) => setTimeout(r, 800));

const failures = [];
const check = (ok, msg) => { console.log(`${ok ? 'ok  ' : 'FAIL'} ${msg}`); if (!ok) failures.push(msg); };
const browser = await chromium.launch();

function watch(page, label) {
  page.on('console', (m) => { if (m.type() === 'error') check(false, `${label}: console error: ${m.text()}`); });
  page.on('pageerror', (e) => check(false, `${label}: page error: ${e.message}`));
  page.on('requestfailed', (r) => check(false, `${label}: request failed: ${r.url()}`));
  page.on('response', (r) => { if (r.status() >= 400) check(false, `${label}: ${r.status()} ${r.url()}`); });
}
async function open(opts, label, url = '/') {
  const ctx = await browser.newContext(opts);
  const page = await ctx.newPage();
  watch(page, label);
  await page.goto(base + url, { waitUntil: 'networkidle' });
  await page.evaluate(() => document.fonts.ready);
  return { ctx, page };
}

try {
  // static views: reduced motion, so the finished frame is what is measured and shot
  for (const [w, h] of [[1440, 900], [375, 812]]) {
    for (const scheme of ['light', 'dark']) {
      const label = `${w}-${scheme}`;
      const { ctx, page } = await open({ viewport: { width: w, height: h }, colorScheme: scheme, reducedMotion: 'reduce' }, label);
      await page.screenshot({ path: `${shots}/${w}x${h}-${scheme}.png`, fullPage: true });
      const res = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa', 'best-practice']).analyze();
      check(res.violations.length === 0, `axe ${label}: ${res.violations.length} violations ${res.violations.map((v) => `${v.id}(${v.nodes.length})`).join(' ')}`);
      const h1 = await page.locator('h1').count();
      check(h1 === 1, `${label}: ${h1} h1`);
      check(await page.locator('.replay').count() === 0, `${label}: no replay control under reduced motion`);
      await ctx.close();
    }
  }

  // alignment: every line of the visible capture is at most `cols` cells wide and the box lines are exactly that
  {
    const { ctx, page } = await open({ viewport: { width: 1440, height: 900 }, reducedMotion: 'reduce' }, 'grid');
    const r = await page.evaluate(() => {
      const cap = document.querySelector('.scroll-100 .cap');
      const cell = parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--cell'));
      const lines = [...cap.querySelectorAll('.ln > .tx')];
      const widths = lines.map((t) => t.getBoundingClientRect().width);
      const box = lines.filter((t) => /^[╭│╰]/.test(t.textContent)).map((t) => t.getBoundingClientRect().width);
      return { cell, cols: +cap.dataset.cols, max: Math.max(...widths), boxed: box.length, boxSame: box.every((w) => Math.abs(w - box[0]) < 0.6) };
    });
    check(r.max <= r.cols * r.cell + 0.6, `grid: widest line ${r.max.toFixed(1)} px <= ${r.cols} cells (${(r.cols * r.cell).toFixed(1)} px)`);
    check(r.boxed >= 10 && r.boxSame, `grid: ${r.boxed} box lines, all the same width`);
    await ctx.close();
  }

  // no horizontal page scroll
  for (const w of [320, 375, 820, 1440]) {
    const { ctx, page } = await open({ viewport: { width: w, height: 800 }, reducedMotion: 'reduce' }, `w${w}`);
    const [sw, iw] = await page.evaluate(() => [document.documentElement.scrollWidth, innerWidth]);
    check(sw <= iw, `scrollWidth ${sw} <= innerWidth ${iw} at ${w}`);
    if ([320, 820].includes(w)) await page.screenshot({ path: `${shots}/${w}.png`, fullPage: true });
    await ctx.close();
  }

  // phone first load, motion allowed: bytes over the wire (uncompressed here, so an upper bound)
  {
    const ctx = await browser.newContext({ viewport: { width: 375, height: 812 }, isMobile: true, hasTouch: true });
    const page = await ctx.newPage();
    watch(page, 'phone-load');
    let bytes = 0;
    const pending = [];
    page.on('response', (r) => pending.push(r.body().then((b) => { bytes += b.length; }, () => {})));
    await page.goto(base + '/', { waitUntil: 'networkidle' });
    await page.waitForTimeout(500);
    await Promise.all(pending);
    check(bytes < 300 * 1024, `375 first load ${(bytes / 1024).toFixed(1)} KB < 300 KB`);
    console.log(`page weight 375: ${(bytes / 1024).toFixed(1)} KB`);
    await ctx.close();
  }

  // JS off
  {
    const { ctx, page } = await open({ viewport: { width: 1440, height: 900 }, javaScriptEnabled: false }, 'nojs');
    check(await page.locator('.scroll-100 .host').count() >= 8, 'JS off: the finished sweep is in the page');
    check(await page.locator('.copy, .replay').count() === 0, 'JS off: no Copy or replay controls');
    await page.screenshot({ path: `${shots}/1440-js-off.png`, fullPage: true });
    await ctx.close();
  }

  // motion on: first view plays by itself; the replay control reruns it; margin note on a host line
  {
    const { ctx, page } = await open({ viewport: { width: 1440, height: 900 }, reducedMotion: 'no-preference', recordVideo: { dir: `${shots}/.video`, size: { width: 1440, height: 900 } } }, 'motion');
    await page.waitForTimeout(1700);
    await page.screenshot({ path: `${shots}/1440-mid-sweep.png` });
    check(/checking/.test(await page.locator('.scroll-100').innerText()), 'motion: a frame mid-sweep still shows hosts checking');
    await page.waitForTimeout(4500);
    const after = await page.locator('.scroll-100 .host').first().innerText();
    check(/up|down/.test(after), `motion: sweep finished on its own (${after.trim().slice(0, 30)}…)`);
    await page.getByRole('button', { name: 'Run the sweep again' }).click();
    await page.waitForTimeout(900);
    const idle = await page.locator('.scroll-100 .host').nth(1).innerText();
    check(/unchecked|never/.test(idle) && !/down/.test(idle), `motion: replay restarts from the idle list (${idle.trim().slice(0, 40)}…)`);
    await page.waitForTimeout(5200);
    // keyboard pass: Tab to a host line, then to Copy
    await page.mouse.move(5, 5);
    await page.locator('.scroll-100').focus();
    for (let i = 0; i < 12 && !(await page.evaluate(() => document.activeElement.classList.contains('host'))); i++) await page.keyboard.press('Tab');
    check(await page.evaluate(() => document.activeElement.classList.contains('host')), 'Tab reaches a host line');
    await page.keyboard.press('Tab');
    await page.waitForTimeout(150);
    const note = await page.locator('.scroll-100 .host:focus .note').innerText();
    check(/^\+\d+\.\d s$/.test(note), `margin note on a focused host line: ${note}`);
    await page.locator('.scroll-100').scrollIntoViewIfNeeded();
    await page.screenshot({ path: `${shots}/1440-keyboard-host.png` });
    await page.evaluate(() => scrollTo(0, 0));
    await page.getByRole('button', { name: 'Copy install command' }).focus();
    await page.screenshot({ path: `${shots}/1440-keyboard-copy.png` });
    await ctx.grantPermissions(['clipboard-read', 'clipboard-write'], { origin: base });
    await page.keyboard.press('Enter');
    await page.waitForTimeout(200);
    check((await page.locator('.install [role=status]').innerText()) === 'Copied', 'Copy announces "Copied"');
    check((await page.locator('.copy').innerText()) === 'Copied', 'Copy button says Copied');
    check((await page.evaluate(() => navigator.clipboard.readText())) === 'brew install --formula hmrdkn-labs/tap/lazyss', 'clipboard holds the brew command');
    await page.screenshot({ path: `${shots}/1440-copied.png` });
    await page.waitForTimeout(2200);
    check((await page.locator('.copy').innerText()) === 'Copy', 'Copy button resets after 2 s');
    const vid = page.video();
    await ctx.close();
    await vid.saveAs(`${shots}/demo.webm`);
  }

  // full-page shots with motion allowed, after the sweep has played: the replay bar is in them
  for (const [w, h, touch] of [[1440, 900, false], [375, 812, true]]) {
    const { ctx, page } = await open({ viewport: { width: w, height: h }, reducedMotion: 'no-preference', isMobile: touch, hasTouch: touch }, `motion-full-${w}`);
    await page.waitForTimeout(6500);
    check(await page.getByRole('button', { name: 'Run the sweep again' }).isVisible(), `${w}: replay bar is visible when motion is allowed`);
    if (touch) {
      await page.locator('.scroll-60 .host').nth(5).tap();
      const note = await page.locator('.hostnote').innerText();
      check(/^db-replica: down, result at \+\d\.\d s$/.test(note), `375: tapping a host line shows "${note}" under the capture`);
    }
    await page.screenshot({ path: `${shots}/${w}x${h}-motion-full.png`, fullPage: true });
    await ctx.close();
  }

  // the first Tab stop is the skip link, and it works
  {
    const { ctx, page } = await open({ viewport: { width: 1440, height: 900 } }, 'skip');
    await page.keyboard.press('Tab');
    check(await page.evaluate(() => document.activeElement.className === 'skip'), 'first Tab stop is the skip link');
    await page.keyboard.press('Enter');
    check(page.url().endsWith('#main'), 'skip link goes to #main');
    await ctx.close();
  }

  // phone: the 60-column capture scrolls sideways inside its own box
  {
    const { ctx, page } = await open({ viewport: { width: 375, height: 812 }, reducedMotion: 'reduce' }, 'phone-scroll');
    const r = await page.evaluate(() => {
      const s = document.querySelector('.scroll-60');
      const cap = s.querySelector('.cap');
      const cell = parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--cell'));
      const fit = Math.floor((s.clientWidth - parseFloat(getComputedStyle(cap).paddingLeft)) / cell);
      return { fit, vis: getComputedStyle(s).display !== 'none', other: getComputedStyle(document.querySelector('.scroll-100')).display, over: s.scrollWidth > s.clientWidth, tab: s.tabIndex, label: s.getAttribute('aria-label') };
    });
    console.log(`375: ${r.fit} of 60 columns visible without scrolling the box`);
    check(r.vis && r.other === 'none' && r.over && r.tab === 0 && !!r.label, `phone: 60-col capture in a labelled, focusable scroll box ${JSON.stringify(r)}`);
    await page.locator('.scroll-60').scrollIntoViewIfNeeded();
    await page.screenshot({ path: `${shots}/375-capture.png` });
    await ctx.close();
  }
} finally {
  await browser.close();
  server.kill();
  rmSync(`${shots}/.video`, { recursive: true, force: true });
}
console.log(failures.length ? `\n${failures.length} FAILED` : '\nall checks passed');
process.exit(failures.length ? 1 : 0);
