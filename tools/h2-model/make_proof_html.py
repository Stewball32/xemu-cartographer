#!/usr/bin/env python3
"""Emit a standalone HTML page that composites the posed Mark VI using the EXACT
same SVG <mask><image>+<rect> technique as spartan-art.ts / emblem-art.ts, over
the real static assets, so we can render it headless (chromium) on Linux and
prove the in-app browser compositor works here."""
import os, sys

STATIC = os.path.abspath(sys.argv[1])  # sveltekit/static
OUT = sys.argv[2]
def f(p): return "file://" + os.path.join(STATIC, p)

def mask(mid, href):
    return (f'<mask id="{mid}" maskUnits="userSpaceOnUse" x="0" y="0" width="100" height="100">'
            f'<image href="{href}" x="0" y="0" width="100" height="100" preserveAspectRatio="xMidYMid meet"/></mask>')

def spartan(uid, pose, primary, secondary):
    return (mask(uid+'-p', f(f'spartan/{pose}_p.png')) + mask(uid+'-s', f(f'spartan/{pose}_s.png')) +
            f'<image href="{f("spartan/"+pose+".png")}" x="0" y="0" width="100" height="100" preserveAspectRatio="xMidYMid meet"/>'
            f'<rect x="0" y="0" width="100" height="100" fill="{primary}" mask="url(#{uid}-p)" style="mix-blend-mode:multiply"/>'
            f'<rect x="0" y="0" width="100" height="100" fill="{secondary}" mask="url(#{uid}-s)" style="mix-blend-mode:multiply"/>')

def emblem(uid, fg_i, bg_i, e_pri, e_sec, pl_pri, pl_sec):
    # plate(): base secondary + primary overlay ; two(): primary+secondary fg
    bgp = f(f'emblems/bg/{bg_i:02d}_p.png')
    fgp = f(f'emblems/fg/{fg_i:02d}_p.png'); fgs = f(f'emblems/fg/{fg_i:02d}_s.png')
    return (mask(uid+'-bp', bgp) + mask(uid+'-fp', fgp) + mask(uid+'-fs', fgs) +
            f'<rect x="0" y="0" width="100" height="100" fill="{pl_sec}"/>'
            f'<rect x="0" y="0" width="100" height="100" fill="{pl_pri}" mask="url(#{uid}-bp)"/>'
            f'<rect x="0" y="0" width="100" height="100" fill="{e_sec}" mask="url(#{uid}-fs)"/>'
            f'<rect x="0" y="0" width="100" height="100" fill="{e_pri}" mask="url(#{uid}-fp)"/>')

CARDS = [
    ("salute", "Red / White",   "#BE2C2C", "#FDFEFF"),
    ("idle",   "Cobalt / Blue", "#416C8F", "#28459B"),
    ("crouch", "Green / Tan",   "#21922F", "#B19256"),
    ("t_pose", "Teal / Orange", "#36747A", "#F57A1F"),
]
cards = ""
for i, (pose, label, pri, sec) in enumerate(CARDS):
    uid = f"sp{i}"; euid = f"em{i}"
    cards += f'''
    <div class="card">
      <div class="fig">
        <svg viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">{spartan(uid,pose,pri,sec)}</svg>
        <svg class="decal" viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">{emblem(euid,7,5,"#FDFEFF","#BE2C2C","#416C8F","#28459B")}</svg>
      </div>
      <div class="cap"><b>{label}</b><span>{pose}</span></div>
    </div>'''

html = f'''<!doctype html><html><head><meta charset="utf-8"><style>
:root{{color-scheme:dark}} body{{margin:0;background:#15181d;font-family:system-ui,sans-serif;color:#cdd3da}}
.row{{display:flex;gap:18px;padding:22px;justify-content:center}}
.card{{background:#1d2127;border:1px solid #2a2f37;border-radius:12px;padding:12px;width:230px}}
.fig{{position:relative;width:206px;height:206px;filter:drop-shadow(0 6px 14px rgba(0,0,0,.5))}}
.fig>svg{{position:absolute;inset:0;width:100%;height:100%}}
.fig>svg.decal{{inset:auto;left:50%;top:31%;width:17%;height:17%;transform:translate(-50%,-50%) rotate(-1deg)}}
.cap{{display:flex;flex-direction:column;align-items:center;margin-top:8px}}
.cap b{{font-size:.95rem}} .cap span{{font-size:.7rem;opacity:.55}}
h1{{text-align:center;font-size:1rem;font-weight:600;margin:18px 0 0;opacity:.8}}
</style></head><body>
<h1>Appearance Studio — real H2 Mark VI, tinted via cc map + emblem decal (browser SVG compositor, Linux)</h1>
<div class="row">{cards}</div>
</body></html>'''
open(OUT, "w").write(html)
print("wrote", OUT)
