#!/usr/bin/env python3
"""Helpers for capture-sweep.sh.

  frames-to-json.py poll    --socket S --session N --outdir D [--press KEY] [--stable 2.0] [--max 10] [--wait-while TEXT]
  frames-to-json.py snap    --socket S --session N --out FILE
  frames-to-json.py build   --capture-dir D --out sweep.json --meta meta.json

`poll` records the pane as raw ANSI (tmux capture-pane -p -e) as fast as tmux answers (the sweep finishes in under a second),
keeping each distinct frame as NNNN_<ms>ms.ansi, ms counted from the moment KEY
was sent (frame 0 is the pane just before the key). `build` converts the kept
frames into the JSON the landing page renders.
"""
import argparse, json, os, re, subprocess, sys, time

def tmux(socket, *args):
    return subprocess.run(["tmux", "-L", socket, *args], capture_output=True, text=True).stdout

def grab(socket, session):
    return tmux(socket, "capture-pane", "-t", session, "-p", "-e")

def cmd_snap(a):
    open(a.out, "w").write(grab(a.socket, a.session))

def cmd_poll(a):
    os.makedirs(a.outdir, exist_ok=True)
    kept = []
    def keep(frame, t_ms):
        kept.append(frame)
        with open(os.path.join(a.outdir, "%04d_%dms.ansi" % (len(kept) - 1, t_ms)), "w") as f:
            f.write(frame)
    last = grab(a.socket, a.session)
    keep(last, 0)
    t0 = time.monotonic()
    if a.press:
        tmux(a.socket, "send-keys", "-t", a.session, a.press)
    changed = t0
    while True:
        time.sleep(0.005)
        now = time.monotonic()
        frame = grab(a.socket, a.session)
        if frame != last:
            keep(frame, int((now - t0) * 1000))
            last, changed = frame, now
        busy = bool(a.wait_while) and a.wait_while in frame
        if (now - changed >= a.stable and not busy) or now - t0 >= a.max:
            break
    print("%s: %d frames, %.1fs" % (a.outdir, len(kept), time.monotonic() - t0))

# --- ANSI -> spans -----------------------------------------------------------
BASE16 = ["000000","cd0000","00cd00","cdcd00","0000ee","cd00cd","00cdcd","e5e5e5",
          "7f7f7f","ff0000","00ff00","ffff00","5c5cff","ff00ff","00ffff","ffffff"]

def idx_to_hex(n):
    if n < 16:
        return "#" + BASE16[n]
    if n < 232:
        n -= 16
        lv = [0, 95, 135, 175, 215, 255]
        return "#%02x%02x%02x" % (lv[n // 36], lv[(n // 6) % 6], lv[n % 6])
    v = 8 + (n - 232) * 10
    return "#%02x%02x%02x" % (v, v, v)

SGR = re.compile(r"\x1b\[([0-9;:]*)m")
CSI_OTHER = re.compile(r"\x1b\[[0-9;:?]*[A-Za-ln-z]|\x1b[()][A-Za-z0-9]|\x1b[=>]")

def parse_line(line):
    st = {"fg": None, "bg": None, "b": False, "dim": False, "rev": False}
    cells = []
    pos = 0
    while pos < len(line):
        m = SGR.match(line, pos)
        if m:
            p = [int(x) if x else 0 for x in re.split(r"[;:]", m.group(1))] or [0]
            i = 0
            while i < len(p):
                c = p[i]
                if c == 0: st.update(fg=None, bg=None, b=False, dim=False, rev=False)
                elif c == 1: st["b"] = True
                elif c == 2: st["dim"] = True
                elif c == 7: st["rev"] = True
                elif c == 22: st["b"] = st["dim"] = False
                elif c == 27: st["rev"] = False
                elif 30 <= c <= 37: st["fg"] = idx_to_hex(c - 30)
                elif 90 <= c <= 97: st["fg"] = idx_to_hex(c - 90 + 8)
                elif c == 39: st["fg"] = None
                elif 40 <= c <= 47: st["bg"] = idx_to_hex(c - 40)
                elif 100 <= c <= 107: st["bg"] = idx_to_hex(c - 100 + 8)
                elif c == 49: st["bg"] = None
                elif c in (38, 48):
                    key = "fg" if c == 38 else "bg"
                    if i + 1 < len(p) and p[i + 1] == 5 and i + 2 < len(p):
                        st[key] = idx_to_hex(p[i + 2]); i += 2
                    elif i + 1 < len(p) and p[i + 1] == 2 and i + 4 < len(p):
                        st[key] = "#%02x%02x%02x" % tuple(p[i + 2:i + 5]); i += 4
                i += 1
            pos = m.end()
            continue
        m = CSI_OTHER.match(line, pos)
        if m:
            pos = m.end()
            continue
        cells.append((line[pos], dict(st)))
        pos += 1
    # trim trailing cells that are blank and unpainted
    while cells and cells[-1][0] == " " and cells[-1][1]["bg"] is None and not cells[-1][1]["rev"]:
        cells.pop()
    spans = []
    for ch, s in cells:
        if spans and all(spans[-1][k] == s[k] for k in s):
            spans[-1]["t"] += ch
        else:
            spans.append({"t": ch, **s})
    return spans

def parse_frame(raw):
    lines = raw.split("\n")
    if lines and lines[-1] == "":
        lines.pop()
    return [parse_line(l) for l in lines]

def frames_in(d):
    out = []
    for name in sorted(os.listdir(d)):
        m = re.match(r"(\d+)_(\d+)ms\.ansi$", name)
        if m:
            out.append({"t_ms": int(m.group(2)), "lines": parse_frame(open(os.path.join(d, name)).read())})
    return out

def cmd_build(a):
    meta = json.load(open(a.meta))
    cd = a.capture_dir
    doc = dict(meta)
    doc["sizes"] = {}
    for size, rows in meta.pop("_sizes").items():
        doc["sizes"][size] = {"cols": int(size), "rows": rows, "frames": frames_in(os.path.join(cd, "frames", size))}
    doc.pop("_sizes", None)
    doc["extra"] = {}
    for name in ("help",):
        p = os.path.join(cd, "extra-%s.ansi" % name)
        if os.path.exists(p):
            doc["extra"][name] = {"lines": parse_frame(open(p).read())}
    json.dump(doc, open(a.out, "w"), ensure_ascii=False, indent=1)
    open(a.out, "a").write("\n")

def main():
    ap = argparse.ArgumentParser()
    sub = ap.add_subparsers(dest="cmd", required=True)
    for name in ("poll", "snap", "build"):
        sp = sub.add_parser(name)
        if name != "build":
            sp.add_argument("--socket", required=True); sp.add_argument("--session", required=True)
        if name == "poll":
            sp.add_argument("--outdir", required=True); sp.add_argument("--press", default="")
            sp.add_argument("--stable", type=float, default=2.0); sp.add_argument("--max", type=float, default=10.0)
            sp.add_argument("--wait-while", default="")
        if name == "snap":
            sp.add_argument("--out", required=True)
        if name == "build":
            sp.add_argument("--capture-dir", required=True); sp.add_argument("--out", required=True); sp.add_argument("--meta", required=True)
    a = ap.parse_args()
    {"poll": cmd_poll, "snap": cmd_snap, "build": cmd_build}[a.cmd](a)

if __name__ == "__main__":
    main()
