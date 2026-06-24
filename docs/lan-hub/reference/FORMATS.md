# Halo CE & Halo 2 — Xbox UDATA save formats (player profiles + gametypes)

Reverse-engineered **offline**, 2026-06-23/24, for the xemu-cartographer LAN hub
(de-risk unknown #2). **No xemu was launched.** All samples were read **read-only**
from the fleet HDD images by converting each `qcow2 → sparse raw` with
`qemu-img convert -O raw` (read-only on the source) and reading the FATX `E:`
partition with **pyfatx**. The source `qcow2` files were never modified.

## Sources (provenance)

| Title | TitleID | qcow2 | What |
|---|---|---|---|
| Halo: Combat Evolved | `0x4D530004` | `containers/.../hdds/_default.qcow2` | 23 gametypes + 1 campaign save under `E:\UDATA\4d530004\` |
| Halo 2 | `0x4D530064` | `~/.local/share/xemu/xemu/hdd2.qcow2` (fleet‑1) | 2 player profiles + 1 gametype under `E:\UDATA\4d530064\` |

The primary `hdd.qcow2` has CE installed but **no saved gametypes** (only
`TitleMeta/TitleImage/SaveImage`). The 23 CE gametypes were verified
**byte-identical** between `_default.qcow2` (read directly) and the prior
`~/xemu-hdd-extract/_default` extraction (23/23 match).

## UDATA layout (extends the cracked `NICKNAME.XBN` work)

```
E:\UDATA\<TitleID>\
  TitleMeta.xbx, TitleImage.xbx, SaveImage.xbx     ← title-level metadata (not saves)
  <SaveDir>\
     SaveMeta.xbx                                  ← display-name sidecar (UNSIGNED)
     <payload file(s)>                             ← the actual save (SIGNED, see §Digest)
```

* **CE** save dir = a human name (e.g. `G-TS 50`, `P-LD50 II`); payload = `blam.lst`
  (gametype) or `blam.sav`+`savegame.bin` (campaign).
* **H2** save dir = a 12-hex-digit id (e.g. `E4CADA6B1E65`); payload = `profile`
  (player profile) or `<modename>` such as `slayer` (gametype).

---

## 1. `SaveMeta.xbx` — display name sidecar (BOTH titles) — **fully cracked, unsigned**

Plain UTF‑16LE text with BOM. No checksum, no signature.

```
FF FE                                  ← UTF-16LE BOM
"Name=" <display name> "\r\n"          ← UTF-16LE
```

Examples: `Name=TS 50`, `Name=Profile: Stew`, `Name=Slayer: team brs`.
Generator: `savemeta_build("TS 25")`. Round-trips byte-identical on all samples.

---

## 2. Halo CE gametype — `blam.lst` — **content cracked**

Fixed **512 bytes**. Little-endian. Verified by diffing **23** real variants
(CTF / Team Slayer / FFA / Oddball / King / Snipers / 1v1 / Race-"practice").

| Offset | Size | Field | Notes / observed values |
|---|---|---|---|
| `0x00` | 24 (`0x18`) | **name** | UTF-16LE, NUL-terminated, fixed 24-byte buffer (≤11 chars) |
| `0x18` | u32 | **game engine** | **1=CTF, 2=Slayer, 3=Oddball, 4=King, 5=Race** |
| `0x1C` | u32 | **teams** | 1 = team game, 0 = free-for-all (FFA/1v1) |
| `0x20` | u32 | **options bitfield** | base `0x22`; **bit0 (+1)=the "R" suffix** (radar/respawn); Snipers sets `0x10`; Practice sets `0x04` |
| `0x24` | u32 | scoring subtype | Slayer=2, CTF/Oddball/King=1, some "R"/WIZARD=0 |
| `0x28`,`0x2C` | u32×2 | reserved | 0 |
| `0x30` | u32 | **time limit** | `300`=“10 min”, `225`=“7.5”, `150`=“5”, `0`=none → **unit = 2 s (minutes×30)** *(scale unconfirmed in-game)* |
| `0x34` | u32 | time limit 2 | secondary timer (e.g. TS50=150, TS100=300) |
| `0x38` | u32 | reserved | 0 |
| `0x3C` | f32 | speed/scale | `1.0` on all samples |
| `0x40` | u32 | **score limit** | CTF captures (3/5), Slayer kills (50/100), 1v1=15, Oddball/King=5, Practice=1 |
| `0x44` | u32 | option 2 | WIZARD=1, Snipers=4, else 0 |
| `0x48` | u32 | respawn/lives | 2 = normal, 0 = practice/training |
| `0x4C` | u32 | **engine-specific union** | CTF=`0x01000000`, Slayer=`0x0101`, King=`1`, Oddball=`0` |
| `0x50`–`0x67` | 24 | zero | |
| **`0x68`–`0x7B`** | **20** | **DIGEST** | content-dependent; **algorithm unresolved** — see §Digest |
| `0x7C`–`0x1FF` | 388 | padding | `0xFF000000`-fill or stale heap; **don't-care**, preserved from template |

The leading bytes are the variant name, so two variants that differ in one
setting (e.g. `CTF 3C 10S` vs `CTF 3C 7S`) differ at exactly `0x30` (time) and the
digest — which is how the table above was derived.

## 3. Halo CE campaign save — `blam.sav` (512 B) + `savegame.bin` (3.5 MB)

`P-LD50 II` is a **campaign** save (level `a10` = Pillar of Autumn), not an MP
player profile. `blam.sav` shares the name+struct shape of `blam.lst` (engine `9`);
`savegame.bin` begins with a **4-byte checksum** (`4F A6 D7 1F`) over the body —
**not** a standard zlib CRC32 (ruled out) → Halo-custom. Out of scope for the
profile/gametype generator, documented for completeness.

> **Halo CE has no standalone multiplayer "player profile" file.** MP player name
> and controller/look settings are not stored as an editable UDATA save the way
> Halo 2 stores them. The CE "appearance/controls" half of the brief therefore
> applies to **Halo 2** (below); CE's editable surface is the **gametype**.

---

## 4. Halo 2 gametype — payload named after the mode (e.g. `slayer`) — **structure mapped (1 sample)**

Sample `slayer` = **324 bytes** ("Slayer: team brs"). Trailing 20-byte digest.

| Offset | Size | Field | Notes |
|---|---|---|---|
| `0x00` | u32 | version | `00 00 00 01` |
| `0x04` | UTF-16LE | **name** | NUL-terminated ("team brs") |
| `0x44` | u32 | mode/teams? | `2` |
| `0x48` | u32 | ? | `0x0FDB` |
| `0x50` | u32 | **score limit** | `0x32` = **50** (slayer kills) |
| … | | other settings | only one sample → not yet diff-mapped |
| `len‑20`..`len‑1` | 20 | **DIGEST** | trailing; unresolved (§Digest) |

> Only one H2 gametype was on the fleet box. To map the remaining fields, capture
> 2–3 more H2 gametypes (vary score, time, mode) and diff exactly as for CE.

## 5. Halo 2 player profile — `profile` — **structure mapped (2 samples)**

Fixed **500 bytes** ("Stew", "Halo0001"). Trailing 20-byte digest.

| Offset | Size | Field | Notes |
|---|---|---|---|
| `0x00`–`0x07` | 8 | header | zero |
| `0x08` | UTF-16LE | **player name** | NUL-terminated, large buffer |
| `0x118`–`0x12F` | 24 | **appearance + controller block** | per-profile bytes (see below) |
| `0x1E0`–`0x1F3` | 20 | **DIGEST** | trailing; unresolved (§Digest) |

Appearance/control bytes that differ between the two profiles (provisional labels —
2 samples; confirm with more profiles or the H2 menu RE):

| Offset | "Stew" | "Halo0001" | likely |
|---|---|---|---|
| `0x118` | 10 | 13 | primary armour colour |
| `0x11A` | 0 | 14 | secondary colour / emblem |
| `0x11B` | 0 | 14 | emblem |
| `0x11D` | 12 | 26 | emblem icon |
| `0x11E` | 3 | 22 | emblem colour |
| `0x12B`,`0x12F` | 0 | `0x40` | controller (sensitivity / toggle) |

These are plain small-int enums — directly settable. For generation, copy the
block from a template profile and patch individual bytes.

---

## 6. The 20-byte DIGEST — the one unresolved item (CE + H2)

Every **payload** save (CE `blam.lst`, H2 `profile`, H2 `slayer`) carries a
**20-byte, content-dependent** field — embedded at `0x68` in CE's fixed-size
record, **trailing** in H2's variable-size records. This is exactly the *"signed
UDATA savegame"* the `NICKNAME.XBN` write-up warned about (NICKNAME itself is
unsigned; these are not).

**Ruled out (tested exhaustively on real samples):**
* plain **SHA-1** over every 4-byte-aligned `[start:end]` range (raw and with the
  digest field zeroed) — no match;
* **MD5(16)+CRC32(4)** split — no match;
* **HMAC-SHA1** under obvious keys (titleid LE/BE, save name, `blam`, `halo`,
  zero/0xFF keys) over plausible content ranges — no match;
* for H2 (clean trailing digest) **SHA-1/HMAC over `[0:digest]`** — no match.

**Therefore:** it is a **keyed or salted 20-byte digest** (20 bytes ⇒ SHA-1
family). The most probable identities:
1. the **Xbox content signature** (`XapiSignatureWork` / `XCalculateSignature*` =
   HMAC-SHA1 under a **per-title** key derived from the Halo XBE certificate +
   kernel secret), or
2. a **Halo-internal salted SHA-1** using a constant baked into the Halo XBE.

Both require a secret we don't yet have offline. The competitive CE pack being a
**portable, distributable set** argues the key is **per-title, not per-console**
(else it couldn't be copied between boxes) — i.e. recoverable from the Halo XBE,
not the console.

**Open question that actually gates generation:** *is the digest verified when
Halo loads the file?* Many Xbox titles write an internal save hash but don't
re-verify it on load. This is a 10-minute runtime check (see `DE-RISK-VERDICT.md`).

## CE vs H2 — differences at a glance

| | Halo CE | Halo 2 |
|---|---|---|
| gametype file | `blam.lst`, fixed **512 B**, name @ `0x00` | mode-named (`slayer`…), **variable**, name @ `0x04` after a version word |
| digest position | embedded `0x68–0x7B` | trailing last 20 B |
| player profile | none (no editable MP profile file) | `profile`, **500 B**, name @ `0x08`, appearance/controls @ `0x118` |
| save dir name | human (`G-…`, `P-…`) | 12-hex id |
| SaveMeta.xbx | identical trivial UTF-16 `Name=…` | identical |
