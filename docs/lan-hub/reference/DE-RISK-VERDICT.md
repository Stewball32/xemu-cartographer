# DE-RISK VERDICT — unknown #2: generate Xbox-acceptable Halo CE/H2 profile & gametype files

> **UPDATE 2026-06-24 (later, offline): the one remaining blocker — the 20-byte digest —
> is now CRACKED.** It is the Original-Xbox roamable save signature:
> `HMAC-SHA1(AuthKey, data)`, `AuthKey = HMAC-SHA1(XboxKey, cert.sig_key)[:16]`. Verified
> byte-exact on all real samples (CE 23/23, H2 3/3); `recompute_digest()` is implemented
> and the generator now emits correctly-signed edited files **fully offline** — no xemu
> probe and no pad-automation fallback needed. The "is it enforced?" question below is now
> moot (our signatures are correct either way). Full details: **`SIGNATURE-CRACK.md`**.
> The sections below are retained as the original verdict for the record.

**Date:** 2026-06-24   **Method:** offline file analysis only (no xemu launched;
qcow2 images read read-only via `qemu-img convert -O raw` + pyfatx; sources untouched).

## Verdict

> **Largely de-risked. The *content* formats are cracked and we can already
> generate field-correct profile & gametype files by template-patching real
> samples. ONE blocker remains before we can assert "the Xbox will accept an
> *edited-settings* file": a 20-byte per-save integrity digest whose algorithm we
> did not crack offline, and whose *enforcement on load* is unverified. That last
> question is a ~10-minute xemu check, deferred per the no-xemu constraint.**

## Scorecard

| Capability | Status | Confidence |
|---|---|---|
| Read/parse CE gametype (`blam.lst`) | ✅ cracked, 23 samples, byte-perfect round-trip | **High** |
| Read/parse H2 gametype | ✅ structure mapped (name, score, digest), 1 sample | Med (more samples → High) |
| Read/parse H2 player profile (name, appearance, controls) | ✅ structure mapped, 2 samples | Med-High |
| `SaveMeta.xbx` (display name) generation | ✅ trivial, unsigned, byte-perfect | **High** |
| Generate file with **unchanged** settings (re-sign-free clone) | ✅ byte-identical to original | **High** |
| Generate file with **edited** settings (name/limits/appearance) | ✅ field-correct; ⚠️ digest not recomputed | **Med** |
| Xbox/Halo **accepts** an edited-settings file | ❓ needs runtime test | **Unknown** |
| Recompute the 20-byte digest from scratch | ❌ algorithm/key not recovered | Low (path known) |

## What "cracked" buys us today (proven, 50/50 checks pass — see `roundtrip_test.py`)

* Lossless parse↔serialize of **every** real sample (CE ×23, H2 ×3, SaveMeta ×8):
  re-serialization is **byte-identical**, i.e. our model accounts for 100% of bytes.
* **Surgical edits:** changing `score_limit`/`time_limit`/`name`/appearance touches
  *only* those bytes; the digest and all padding are preserved untouched.
* **Fresh files generated** (`generated/`) and re-parsed correctly: a CE "TS 25"
  gametype, an H2 "CARTOG" profile, an H2 gametype.

## The one unresolved thing — the 20-byte digest

A content-dependent 20-byte field on each payload save (CE `blam.lst` @`0x68`;
H2 `profile`/gametype trailing). Exhaustively **not** plain SHA-1/MD5/CRC and
**not** HMAC-SHA1 under obvious keys ⇒ a **keyed/salted** digest (almost certainly
the Xbox content signature, HMAC-SHA1 under a **per-title** key from the Halo XBE,
or a Halo-internal salted SHA-1). Details + everything ruled out: `FORMATS.md §6`.

Two independent ways forward, in priority order:

### A. Determine whether the digest is even enforced (do this first — cheap, decisive)
If Halo doesn't re-verify the digest on load (common for internal save hashes),
then template-patch already produces **fully accepted** files and unknown #2 is
**fully closed**. Runtime probe (next time xemu is up):
1. Pick a real gametype, e.g. clone `_default.qcow2`'s `G-TS 50`.
2. With **xemu off**, write a copy of the HDD; use the generator to set
   `score_limit=25` **without** touching the digest; place it back as a new save
   dir (`G-TS 25` + matching `SaveMeta.xbx`). (Use the repo's raw-roundtrip recipe
   from `SYSTEM-INFO.md §7`; never edit a qcow2 while xemu runs.)
3. Boot xemu → Halo → Multiplayer → Edit Game Types. **Accepted & shows 25 kills?**
   digest is *not* enforced → ship template-patch. **Rejected/again 50/"corrupt"?**
   it *is* enforced → go to B.

### B. If enforced — recover the digest (still very feasible)
* **Best / lowest-effort:** don't synthesize at all — **drive the in-game menu**
  with the existing **pad-fleet automation** (`scripts/fleet/padfleet.py` + FIFO
  vocabulary already in the repo) to create the exact variant in-game; the engine
  writes a correct digest natively. Then image it. This **completely sidesteps**
  the digest and fits the cartographer hub's existing control plane.
* **Full crack:** extract the Halo title signing key from the XBE certificate
  (`scripts/inventory_xbes.py` already parses XBE certs) and implement the Xbox
  HMAC-SHA1 derivation; drop it into `recompute_digest()` in `halo_save_formats.py`
  (the hook is already wired). Higher effort; only needed if fully-synthetic,
  in-bulk generation is required.

## Recommendation

Treat unknown #2 as **de-risked to "yes, with a known finishing step."** The
content formats are solved and we can read/clone/parameterize every field. Before
relying on **edited** files in production, run probe **A** (10 min) to learn if the
digest is enforced; if it is, prefer fallback **B-menu-drive** (reuses fleet
automation) over cracking the HMAC. No blocker requires re-opening the binary RE.

## Residual risks / caveats
* Time-limit unit (`×30`) and CE `0x24/0x44/0x48/0x4C` semantics are inferred from
  value patterns, not in-game-confirmed; the generator stores raw values so this
  doesn't affect cloning, only human-friendly labels.
* H2 field map is partial (1 gametype, 2 profiles). Capture a few more and diff to
  finish labelling (mechanical).
* Digest enforcement unknown (the gating item above).
