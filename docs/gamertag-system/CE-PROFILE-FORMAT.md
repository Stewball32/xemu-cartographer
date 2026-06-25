# Halo: CE (Xbox) player profile — definitive format

**Correction of record.** Halo: Combat Evolved on the original Xbox **does have
in‑game player profiles**, exactly parallel to Halo 2. The earlier "CE has no MP
profile / CE is name‑only (the console name)" conclusion was wrong — it reasoned
from a hard drive that simply had no CE profile created, plus a "name+armor+
controls is Halo PC, not Xbox" misread. This document is the empirical, signed,
byte‑verified replacement, captured by booting stock Halo CE in xemu and creating
a profile.

**Method:** booted `Halo CE.iso` in an isolated xemu (fresh qcow2 overlay over the
clean base, private EEPROM, FIFO‑driven virtual pad that self‑cleans). In‑game:
Settings → **SELECT PROFILE TO EDIT** → CREATE NEW → named it, set color **Red**,
set thumbstick+button layouts to **Southpaw** → **SAVE CHANGES** → clean QMP
shutdown. Then read the FATX `E:` partition read‑only with pyfatx. Two real
samples diffed (**Stew** existing vs **New001** created), plus the pre‑existing
campaign profile from the competitive pack.

---

## 1. Where CE stores the profile

```
E:\UDATA\4d530004\<12-hex-id>\
    blam.sav        512 bytes   ← THE PLAYER PROFILE  (name + color + controls, SIGNED)
    savegame.bin    3,670,016 B ← that profile's campaign checkpoint (auto-created)
    SaveMeta.xbx    ~24-28 B    ← UTF-16 "Name=<name>" display sidecar (UNSIGNED)
```

* It is **UDATA** (per‑user save), **not** TDATA. CE's `TDATA\4d530004\` is an
  empty directory.
* The save‑dir name is an **auto‑generated 12‑hex‑digit id**, same convention as
  H2 (observed: `122A17771B9E`, `007400651398`). The competitive pack's
  human names (`P-LD50 II`, `G-TS 50`) were **manual renames** by the pack author,
  which is what made `P-LD50 II` look like "just a campaign save."
* **`blam.sav` IS the profile.** Creating a profile also writes a default
  `savegame.bin` (campaign = start of *The Pillar of Autumn*) even though no
  campaign was played — so a dir holding `blam.sav` **and** `savegame.bin` is a
  *profile with its campaign save*, not "only a campaign save." That pairing is
  exactly what the earlier pass saw and mislabeled.

---

## 2. `blam.sav` layout — 512 bytes, little‑endian

Only the **first 48 bytes (0x00–0x2F) are signed/meaningful**; the rest is
uninitialised heap padding (don't‑care, preserved or zeroed on generate).

| Offset | Size | Field | Evidence / values |
|---|---|---|---|
| `0x00` | 24 (`0x18`) | **name** | UTF‑16LE, NUL‑terminated, 24‑byte buffer ⇒ **≤ 11 chars** (`Stew`, `New001`) |
| `0x18` | u32 | **MP armor color** | Stew `0x0A`, **New001 = `0x02` = Red** (matches c20 enum white=0, red=2, blue=3) |
| `0x1C`–`0x27` | 12 | advanced block | `0` in both MP samples (ADVANCED SETUP left at defaults) |
| `0x28` | u8 | **thumbstick layout preset** | `0`=Default, **`1`=Southpaw** (New001=1, Stew=0) |
| `0x29` | u8 | **button layout preset** | `0`=Default, **`1`=Southpaw** (New001=1, Stew=0) |
| `0x2A` | u8 | advanced setting (unmapped) | `0x03` in both new samples; `0x0A` in the pack's profile ⇒ **varies** (look‑sensitivity / invert / vibration candidate) |
| `0x2B`–`0x2F` | 5 | reserved | `0` (a per‑profile flag at `0x2C` seen `=1` in one sample) |
| **`0x30`–`0x43`** | **20** | **DIGEST** | HMAC‑SHA1; **see §3** |
| `0x44`–`0x1FF` | 444 | unsigned tail | leaked heap/kernel pointers; not canonical, differs per save |

Signed‑region diff (Stew vs New001), everything outside the name:

```
0x18: 0a -> 02   (color: white-ish -> red)
0x28: 00 -> 01   (thumbstick: Default -> Southpaw)
0x29: 00 -> 01   (buttons:    Default -> Southpaw)
```

Nothing else in 0x00–0x2F changed. That is the whole editable surface I varied,
and it landed in three bytes — a clean map.

In‑game the profile editor (**EDIT PROFILE SETTINGS**) exposes: **Change Name**,
**Controller Setup** (Thumbstick + Button presets → `0x28`/`0x29`), **Advanced
Setup** (sensitivity / invert look / invert flight / vibration / auto‑center →
the `0x1C–0x2F` region, individual offsets not yet mapped), **Change Color**
(`0x18`), **Save Changes**.

---

## 3. The 20‑byte digest — already cracked, just a different offset

The CE profile uses the **same Original‑Xbox roamable save signature** already
implemented in `internal/halosave/digest.go` — **no new crypto work**:

```
digest(20B) = HMAC-SHA1(AuthKey, blam.sav[:0x30])          # signs the first 48 bytes
AuthKey(16)  = HMAC-SHA1(XboxGlobalKey, CE_title_sig_key)[:16]
            = 5770e155a1c75fa9830b141896544428             # the CE per-title key
```

The **only** difference from the CE gametype is the offset/length:

| file | signed range | digest at |
|---|---|---|
| `blam.lst` (gametype) | `[0x00:0x68]` (104 B) | `0x68` |
| **`blam.sav` (profile)** | **`[0x00:0x30]` (48 B)** | **`0x30`** |

**Verified byte‑exact** on every real CE profile sample: `Stew` ✓, `New001` ✓,
and the pack's pre‑existing profile ✓ — all three reproduce their stored digest
from `HMAC-SHA1(CE_AuthKey, blam[:0x30])`. The existing Go call already produces
it: `RecomputeDigest(buf, 0x30, 20, "ce")`.

**Generator proven:** I template‑patched `New001` → name `CARTOG`, color blue
(`0x03`), controls Default, re‑signed at `0x30`; the new file self‑verifies. That
is the editor's exact output path (template‑patch + re‑sign), identical in shape
to the H2 generator.

---

## 4. CE profile vs H2 profile

| | **Halo CE — `blam.sav`** | **Halo 2 — `profile`** |
|---|---|---|
| path | `UDATA\4d530004\<hex>\blam.sav` | `UDATA\4d530064\<hex>\profile` |
| size | **512 B** fixed | 500 B fixed |
| name | `@0x00`, UTF‑16LE, 24‑B buf (**≤11**) | `@0x08`, UTF‑16LE, larger buf |
| appearance | **1 color enum** `@0x18` (single color — CE has no 2‑tone) | armor + emblem block `@0x118` |
| controls | **2 preset bytes** `@0x28`/`@0x29` + advanced region | controller bytes in the `0x118` block |
| digest | `@0x30`, signs `[0:0x30]` | `@0x1E0` (trailing), signs `[0:0x1E0]` |
| signing key | **same per‑title roamable HMAC‑SHA1** | same scheme, H2 per‑title key |
| dir id | 12‑hex auto | 12‑hex auto |
| siblings | `savegame.bin` (campaign) + `SaveMeta.xbx` | `SaveMeta.xbx` |

Same family, same signature crack, same UTF‑16 name + auto‑hex dir. CE is simpler:
one color enum and two control presets vs H2's appearance/emblem block.

---

## 5. What this means for the gamertag system (CE side corrections)

The gamertag is **one in‑game name driving both profiles** — and the CE side is a
*real, signed UDATA save just like H2*, not "name‑only / the console name."

1. **`docs/lan-hub/reference/FORMATS.md` §3** — "Halo CE has no standalone
   multiplayer player profile file" is **false**. `blam.sav` is the profile
   (name `@0x00`, color `@0x18`, controls `@0x28/0x29`, digest `@0x30`). The
   `P-LD50 II` dir was a real profile, not merely a campaign save.

2. **`docs/gamertag-system/README.md` → "The CE profile is name‑only (the console
   name)"** — replace. The CE profile is **not** `E:\UDATA\NICKNAME.XBN`.
   `NICKNAME.XBN` is the **Xbox console / system‑link name** (a separate, real,
   dashboard‑level thing); the **CE *game* profile** is `blam.sav` and carries
   name + color + controls, signed, exactly parallel to H2. The gamertag should
   drive the **`blam.sav` name** (the in‑game MP name) the same way it drives the
   H2 `profile` name. Setting `NICKNAME.XBN` too is fine, but it is *not* the CE
   MP profile.

3. **`ce_profiles` (schema + generate hook)** — it currently has *no editable
   fields* and emits `NICKNAME.XBN`. It should instead generate
   `UDATA\4d530004\<id>\blam.sav` (+ `SaveMeta.xbx`, and likely a default
   `savegame.bin` so CE lists it), **signed via `RecomputeDigest(buf,0x30,20,"ce")`**,
   with editable fields:
   * **color** (u32 `@0x18`, enum)
   * **thumbstick preset** (u8 `@0x28`) and **button preset** (u8 `@0x29`)
   * (later) the advanced `0x1C–0x2F` settings once individually mapped.
   A `CEProfileParse`/`CEProfileBuild` pair mirroring `h2profile.go` is the clean
   addition (offsets above; reuse the existing digest function).

4. **The 11‑char gamertag cap is correct** and confirmed: the CE name buffer is
   the 24‑byte field `0x00–0x17`, so ≤ 11 UTF‑16 chars — the tighter of the two
   games, exactly as the README already states.

---

## 6. Open / not yet mapped (small follow‑ups)

* **Advanced Setup bytes** (look sensitivity, invert look, invert flight,
  vibration, auto‑center) live in `0x1C–0x2F` but weren't individually mapped — I
  varied color + control presets only. `0x2A` and `0x2C` are already known to vary.
  One more capture that sweeps Advanced Setup pins them down.
* **Color enum** — confirmed Red = `0x02` (and c20's white=0/blue=3); the full
  carousel index→name table just needs enumerating.
* **Is `savegame.bin` required** for CE to list a profile, or do `blam.sav` +
  `SaveMeta.xbx` suffice? CE auto‑creates it; a generated‑profile‑only test would
  confirm whether the hub must ship a stub campaign save.

## 7. Artifacts (in `ce-profile-samples/`)

* `stew_blam.sav`, `new001_blam.sav` — the two real captured profiles (both
  signature‑valid).
* `generated_CARTOG_blam.sav` — a hub‑style generated+re‑signed profile (proves
  the editor path).
* `ingame_profile_select.png`, `ingame_edit_profile_settings.png`,
  `ingame_color_carousel.png` — in‑game proof of the profile system.

> Scope note: the cartographer hub, its branches, and other in‑flight tasks were
> not modified. Capture ran in a throwaway xemu overlay; the virtual pad was shut
> down and removed. This file is the report; applying the §5 code/doc edits is a
> deliberate next step.
