// Screenshots for the visual review. Writes into <dir>:
//   1440-00.png (opening 1440x900), 1440-01.png ... (each next 900 px scroll, to the bottom),
//   390-00.png (opening 390x844), 390-full.png, 1440-reduced-motion.png, 1440-js-off.png
//   node scripts/shots.mjs <dir>
//   BASE_URL=https://lazyss.hamardikan.com node scripts/shots.mjs <dir>   shoots a live site
// Shots with motion on wait until the page stops changing (replay finished) or 12 s.
import { createRequire } from 'node:module';
import { homedir } from 'node:os';
import { spawn } from 'node:child_process';
import { createServer } from 'node:net';
import { mkdirSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(`${homedir()}/.cache/lazyss-pw/`);
const { chromium } = require('playwright');

if (!process.argv[2]) { console.error('usage: node scripts/shots.mjs <dir>'); process.exit(2); }
const dir = resolve(process.argv[2]);
mkdirSync(dir, { recursive: true });
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

const browser = await chromium.launch();
async function open(opts) {
  const ctx = await browser.newContext(opts);
  const page = await ctx.newPage();
  await page.goto(base + '/', { waitUntil: 'networkidle' });
  await page.evaluate(() => document.fonts.ready);
  return { ctx, page };
}
// resolves once the DOM has been quiet for 1.5 s, or after 12 s
const settle = (page) => page.evaluate(() => new Promise((done) => {
  let quiet;
  const finish = () => { mo.disconnect(); clearTimeout(cap); done(); };
  const mo = new MutationObserver(() => { clearTimeout(quiet); quiet = setTimeout(finish, 1500); });
  mo.observe(document.documentElement, { subtree: true, childList: true, characterData: true, attributes: true });
  quiet = setTimeout(finish, 1500);
  const cap = setTimeout(finish, 12000);
}));
// scrolls to the bottom in steps so lazy images load, then back to the top
async function loadLazy(page) {
  const h = await page.evaluate(() => document.documentElement.scrollHeight);
  for (let y = 0; y < h; y += 600) { await page.evaluate((v) => scrollTo(0, v), y); await page.waitForTimeout(150); }
  await page.evaluate(() => scrollTo(0, 0));
  await page.waitForLoadState('networkidle');
  await page.evaluate(() => Promise.all([...document.images].map((i) => i.complete ? 0 : i.decode().catch(() => {}))));
}

try {
  {
    const { ctx, page } = await open({ viewport: { width: 1440, height: 900 } });
    await settle(page);
    await loadLazy(page);
    const h = await page.evaluate(() => document.documentElement.scrollHeight);
    let n = 0;
    for (let y = 0; ; y += 900, n++) {
      await page.evaluate((v) => scrollTo(0, v), y);
      await page.waitForTimeout(400);
      await page.screenshot({ path: `${dir}/1440-${String(n).padStart(2, '0')}.png` });
      if (y + 900 >= h) break;
    }
    console.log(`1440: ${n + 1} screens`);
    await ctx.close();
  }
  {
    const { ctx, page } = await open({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true });
    await settle(page);
    await page.screenshot({ path: `${dir}/390-00.png` });
    await loadLazy(page);
    await page.screenshot({ path: `${dir}/390-full.png`, fullPage: true });
    await ctx.close();
  }
  {
    const { ctx, page } = await open({ viewport: { width: 1440, height: 900 }, reducedMotion: 'reduce' });
    await loadLazy(page);
    await page.screenshot({ path: `${dir}/1440-reduced-motion.png`, fullPage: true });
    await ctx.close();
  }
  {
    const { ctx, page } = await open({ viewport: { width: 1440, height: 900 }, javaScriptEnabled: false });
    await page.screenshot({ path: `${dir}/1440-js-off.png`, fullPage: true });
    await ctx.close();
  }
} finally {
  await browser.close();
  server?.kill();
}
console.log(`shots in ${dir}`);
