// Draws public/og.png (1200×630) from the real rendered sweep, and public/apple-touch-icon.png from favicon.svg.
// Needs the site served locally and Playwright, which is not a dependency of this repo:
//   npm i --prefix ~/.cache/lazyss-pw playwright && npx --prefix ~/.cache/lazyss-pw playwright install chromium
//   python3 -m http.server 8791 -d public & NODE_PATH=~/.cache/lazyss-pw/node_modules node scripts/og.mjs [http://127.0.0.1:8791]
import { createRequire } from 'node:module';
import { homedir } from 'node:os';
const require = createRequire(`${homedir()}/.cache/lazyss-pw/`);
const { chromium } = require('playwright');

const base = (process.argv[2] ?? 'http://127.0.0.1:8791').replace(/\/$/, '');
const out = new URL('../public/', import.meta.url).pathname;

const browser = await chromium.launch();
try {
  const page = await browser.newPage({ viewport: { width: 1200, height: 630 }, colorScheme: 'light', reducedMotion: 'reduce' });
  await page.goto(`${base}/`, { waitUntil: 'networkidle' });
  await page.setViewportSize({ width: 1400, height: 900 }); // the 100-column capture
  await page.evaluate(() => document.fonts.ready);
  const html = await page.evaluate(() => {
    const cap = document.querySelector('.scroll-100').cloneNode(true);
    cap.querySelectorAll('.note').forEach((n) => n.remove());
    cap.removeAttribute('tabindex');
    cap.querySelectorAll('[tabindex]').forEach((n) => n.removeAttribute('tabindex'));
    return cap.outerHTML;
  });
  await page.setViewportSize({ width: 1200, height: 630 });
  await page.evaluate((cap) => {
    document.body.innerHTML = `
      <div id="og" style="position:fixed;inset:0;padding:44px 52px 0;overflow:hidden">
        <div style="display:flex;align-items:baseline;justify-content:space-between">
          <p class="mark" style="margin:0"><span class="wordmark" style="font-size:22px">lazyss</span></p>
          <p class="label" style="margin:0">lazyss.hamardikan.com</p>
        </div>
        <h1 style="font-size:46px;margin:14px 0 22px;max-width:none">A terminal cockpit for your SSH and SSM hosts.</h1>
        <div style="height:452px;overflow:hidden">${cap}</div>
      </div>`;
    const s = document.querySelector('#og .scroll');
    s.style.display = 'block';
    s.style.overflow = 'hidden';
  }, html);
  await page.evaluate(() => document.fonts.ready);
  await page.screenshot({ path: `${out}og.png` });
  console.log('wrote og.png');

  const icon = await browser.newPage({ viewport: { width: 180, height: 180 } });
  await icon.setContent(`<body style="margin:0"><img src="${base}/favicon.svg" width="180" height="180" alt=""></body>`);
  await icon.waitForLoadState('networkidle');
  await icon.screenshot({ path: `${out}apple-touch-icon.png` });
  console.log('wrote apple-touch-icon.png');
} finally {
  await browser.close();
}
