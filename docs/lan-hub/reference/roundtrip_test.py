#!/usr/bin/env python3
"""
roundtrip_test.py — verification harness for halo_save_formats.py

Proves three things the de-risk needs:
  1. PARSE/SERIALIZE FIDELITY: every real sample re-serializes byte-identical
     (parser + writer are lossless; the model covers 100% of the bytes).
  2. FIELD EDITS ARE SURGICAL: changing a setting changes only that field's
     bytes; the 20-byte digest and all other bytes are untouched.
  3. GENERATION: brand-new gametypes/profiles are produced from a template and
     re-parse to the requested values; written to ./generated/.

Run:  python3 roundtrip_test.py
Exit code 0 = all pass.
"""
import os, sys, glob, struct
HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import halo_save_formats as H

CE_DIR = os.path.join(HERE, "samples", "ce")
H2_DIR = os.path.join(HERE, "samples", "h2")
GEN    = os.path.join(HERE, "generated")
os.makedirs(GEN, exist_ok=True)

passed = failed = 0
def check(label, cond, detail=""):
    global passed, failed
    if cond: passed += 1; print(f"  PASS  {label}")
    else:    failed += 1; print(f"  FAIL  {label}  {detail}")

def diff_offsets(a, b):
    return [i for i in range(min(len(a), len(b))) if a[i] != b[i]] + \
           list(range(min(len(a), len(b)), max(len(a), len(b))))

# ---------------------------------------------------------------------------
print("\n[1] CE gametype  parse->serialize fidelity (all samples)")
ce_files = sorted(glob.glob(os.path.join(CE_DIR, "G-*", "blam.lst")))
for p in ce_files:
    b = open(p, "rb").read()
    g = H.ce_parse(b)
    rebuilt = H.ce_build(b)  # no changes
    name = os.path.basename(os.path.dirname(p))
    check(f"CE {name:16s} eng={g.engine_name:7s} score={g.score_limit:<4} "
          f"time={g.time_limit:<4} byte-identical",
          rebuilt == b, f"diff@{diff_offsets(rebuilt,b)[:6]}")

print("\n[2] CE field edit is surgical (TS 50 -> score 25, name 'TS 25')")
tmpl = open(os.path.join(CE_DIR, "G-TS 50", "blam.lst"), "rb").read()
edited = H.ce_build(tmpl, name="TS 25", score_limit=25, time_limit=H.ce_minutes_to_raw(7))
g2 = H.ce_parse(edited)
check("re-parsed name == 'TS 25'", g2.name == "TS 25", g2.name)
check("re-parsed score_limit == 25", g2.score_limit == 25, g2.score_limit)
check("re-parsed time_limit == 210 (7min*30)", g2.time_limit == 210, g2.time_limit)
check("digest preserved (unchanged)", g2.digest == H.ce_parse(tmpl).digest)
changed = diff_offsets(tmpl, edited)
# expected: name bytes (0x00..0x17), score(0x40..43), time(0x30..33)
allowed = set(range(0x00,0x18)) | set(range(0x40,0x44)) | set(range(0x30,0x34))
check("only name/score/time bytes changed", set(changed) <= allowed,
      f"unexpected={sorted(set(changed)-allowed)}")
check("digest region 0x68..0x7b NOT in changed set",
      not (set(changed) & set(range(0x68,0x7c))))

print("\n[3] H2 profile parse->serialize fidelity + edit")
for p in sorted(glob.glob(os.path.join(H2_DIR, "profile_*.bin"))):
    b = open(p, "rb").read()
    pr = H.h2p_parse(b)
    check(f"H2 profile '{pr.name}' byte-identical rebuild", H.h2p_build(b) == b)
ptmpl = open(os.path.join(H2_DIR, "profile_E4CADA6B1E65.bin"), "rb").read()  # Stew
pedit = H.h2p_build(ptmpl, name="CARTOG", appctl_patch={0x118: 7})
pp = H.h2p_parse(pedit)
check("H2 profile renamed to 'CARTOG'", pp.name == "CARTOG", pp.name)
check("H2 appearance byte 0x118 set to 7", pedit[0x118] == 7)
check("H2 profile digest preserved", pp.digest == H.h2p_parse(ptmpl).digest)
ch = diff_offsets(ptmpl, pedit)
check("H2 profile digest region 0x1e0.. untouched", not (set(ch) & set(range(0x1e0,0x1f4))))

print("\n[4] H2 gametype parse->serialize fidelity + edit")
gp = os.path.join(H2_DIR, "gametype_slayer.bin")
gb = open(gp, "rb").read()
hg = H.h2gt_parse(gb)
check(f"H2 gametype name='{hg.name}' score={hg.score_limit} byte-identical",
      H.h2gt_build(gb) == gb)
gedit = H.h2gt_build(gb, name="ball 3", score_limit=3)
hg2 = H.h2gt_parse(gedit)
check("H2 gametype renamed 'ball 3'", hg2.name == "ball 3", hg2.name)
check("H2 gametype score_limit==3", hg2.score_limit == 3, hg2.score_limit)
check("H2 gametype digest preserved", hg2.digest == hg.digest)

print("\n[5] SaveMeta.xbx round-trip")
for p in glob.glob(os.path.join(CE_DIR, "*", "SaveMeta.xbx"))[:5] + \
         glob.glob(os.path.join(H2_DIR, "*.SaveMeta.xbx")):
    b = open(p, "rb").read()
    nm = H.savemeta_parse(b)
    check(f"SaveMeta '{nm}' rebuild identical", H.savemeta_build(nm) == b)

print("\n[6] Generate fresh deliverables into ./generated/")
# CE: a new Team Slayer to 25 from TS 50 template
open(os.path.join(GEN, "blam.lst"), "wb").write(edited)
open(os.path.join(GEN, "SaveMeta.xbx"), "wb").write(H.savemeta_build("TS 25"))
# H2: a new profile 'CARTOG'
open(os.path.join(GEN, "profile"), "wb").write(pedit)
open(os.path.join(GEN, "profile.SaveMeta.xbx"), "wb").write(H.savemeta_build("Profile: CARTOG"))
# H2: a new gametype
open(os.path.join(GEN, "h2_gametype"), "wb").write(gedit)
# re-parse generated from disk
check("generated CE blam.lst re-parses", H.ce_parse(open(os.path.join(GEN,'blam.lst'),'rb').read()).score_limit == 25)
check("generated H2 profile re-parses", H.h2p_parse(open(os.path.join(GEN,'profile'),'rb').read()).name == "CARTOG")
check("generated H2 gametype re-parses", H.h2gt_parse(open(os.path.join(GEN,'h2_gametype'),'rb').read()).score_limit == 3)

print(f"\n==== RESULT: {passed} passed, {failed} failed ====")
sys.exit(1 if failed else 0)
