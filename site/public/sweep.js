// The only script. Two jobs: a Copy button beside the install command, and "print": a replay of the recorded
// sweep at its real timings. The page is complete without either (the finished frame is in the HTML), and the
// replay is not offered under prefers-reduced-motion.
(() => {
  const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;

  // --- Copy ---------------------------------------------------------------
  const install = document.getElementById('install');
  const cmd = document.getElementById('brew');
  if (install && cmd) {
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'copy';
    btn.textContent = 'Copy';
    btn.setAttribute('aria-label', 'Copy install command');
    const live = document.createElement('span');
    live.className = 'vh';
    live.setAttribute('role', 'status');
    install.append(btn, live);
    let timer;
    const done = () => {
      btn.textContent = 'Copied';
      live.textContent = 'Copied';
      clearTimeout(timer);
      timer = setTimeout(() => { btn.textContent = 'Copy'; live.textContent = ''; }, 2000);
    };
    btn.addEventListener('click', async () => {
      const text = cmd.textContent;
      try {
        await navigator.clipboard.writeText(text);
        done();
      } catch {
        const r = document.createRange();
        r.selectNodeContents(cmd);
        const sel = getSelection();
        sel.removeAllRanges();
        sel.addRange(r);
        try { if (document.execCommand('copy')) done(); } finally { sel.removeAllRanges(); }
      }
    });
  }

  const listing = document.getElementById('listing');

  // --- Print --------------------------------------------------------------
  if (reduce || !listing) return;

  const wide = matchMedia('(min-width: 1280px)');
  const key = () => (wide.matches ? '100' : '60');
  const lead = 700; // ms of the idle list before G is pressed
  let data;
  let timers = [];
  let tick;

  const bar = document.createElement('div');
  bar.className = 'replay';
  const button = document.createElement('button');
  button.type = 'button';
  button.textContent = 'Run the sweep again';
  const clock = document.createElement('span');
  clock.className = 'clock';
  clock.setAttribute('aria-hidden', 'true');
  const status = document.createElement('span');
  status.className = 'vh';
  status.setAttribute('role', 'status');
  bar.append(button, clock, status);
  listing.after(bar);

  const secs = (ms) => (ms / 1000).toFixed(1);
  const load = async () => (data ??= await (await fetch('/data/replay.json')).json());

  function lines() {
    const cap = listing.querySelector(`.scroll-${key()} .cap`);
    return cap ? [...cap.querySelectorAll('.ln > .tx')] : [];
  }
  function apply(tx, frame, strike) {
    for (const [i, html] of Object.entries(frame.c)) {
      const el = tx[i];
      if (!el) continue;
      el.innerHTML = html;
      if (strike) {
        el.classList.remove('strike');
        void el.offsetWidth; // restart the animation
        el.classList.add('strike');
      }
    }
  }

  async function play() {
    let d;
    try { d = await load(); } catch { return; }
    timers.forEach(clearTimeout);
    clearInterval(tick);
    timers = [];
    status.textContent = '';
    const tx = lines();
    const frames = d[key()].frames;
    const runMs = d[key()].run_ms;
    apply(tx, frames[0], false);
    const t0 = performance.now() + lead;
    clock.textContent = `0.0 s of ${secs(d[key()].run_ms)} s`;
    for (const f of frames.slice(1)) timers.push(setTimeout(() => apply(tx, f, true), lead + f.t));
    // the times print when their rows land: the margin notes (wide) or the list under the capture (narrow)
    const times = [...listing.querySelectorAll(wide.matches ? '.scroll-100 .note[data-ms]' : '.order li[data-ms]')];
    for (const el of times) {
      el.classList.add('pending');
      el.classList.remove('strike');
      timers.push(setTimeout(() => {
        el.classList.remove('pending');
        void el.offsetWidth;
        el.classList.add('strike');
      }, lead + Number(el.dataset.ms)));
    }
    tick = setInterval(() => {
      const t = Math.min(Math.max(performance.now() - t0, 0), runMs);
      clock.textContent = `${secs(t)} s of ${secs(runMs)} s`;
      if (t >= runMs) {
        clearInterval(tick);
        clock.textContent = `Run length ${secs(runMs)} s`;
        status.textContent = 'Sweep finished';
      }
    }, 100);
  }

  load().then((d) => { clock.textContent = `Run length ${secs(d[key()].run_ms)} s`; }, () => { bar.remove(); });
  button.addEventListener('click', play);

  // once, on first view
  if ('IntersectionObserver' in window) {
    const io = new IntersectionObserver((entries) => {
      if (entries.some((e) => e.isIntersecting)) { io.disconnect(); play(); }
    }, { threshold: 0.4 });
    io.observe(listing);
  }
})();
