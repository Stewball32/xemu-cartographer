# Halo 2 two-instance System Link — SOLVED (2026-07-02)

Goal: get two xemu instances into a shared **Halo 2** System Link lobby. **DONE** —
both instances are in the same live match (screenshot-verified). The fix was a
**distinct eeprom serial per instance**; details below.

## ✅ FIX — distinct eeprom serial (verified 2026-07-02)

The networking always worked (see the diagnosis below). The block was H2 treating
the two instances as the **same console** and filtering the host's game off the
client's list, because both clone `hdd-ceprof` (same console name + profile) **and
share the eeprom serial `757934831097`** (only the MAC differed). CE tolerated this;
H2 does not.

**Fix:** give instance 2 a DISTINCT eeprom serial (`757934830002`) via
`make_h2sl_eeproms.py` (patches `0x34` + recomputes the factory checksum at `0x30`),
and point the launcher at `eeprom-h2sl-1.bin` / `eeprom-h2sl-2.bin`. On relaunch,
instance 2's AVAILABLE GAMES immediately listed **"Halo0001 — Pregame (open)"**, it
joined, and both consoles showed **"2 player(s) in party"** in one System Link Custom
Game (`Snake` + `Halo0001`, team BRs on Turf) — verified live. The serial alone was
sufficient (no console-name or profile change needed).

Bring-up: `make_h2sl_eeproms.py` once, then `udp_reflector.py --bind 9975,9976 --refl
9985,9986` + `launch-h2-instance.sh 1` + `launch-h2-instance.sh 2`; drive both
`Don't Sign In → SYSTEM LINK → profile → AVAILABLE GAMES`, host `Create New Game` on
1, `join` the listed game on 2, host `START GAME`.

---

## Original diagnosis (why frames crossed but the game wasn't listed)

The **networking is working** (both instances discover-broadcast to each other over
the reflector), but the H2 client **did not list** the host's game — an H2
**application-level self-filter** because the two instances are clones of one console.

## Rig (extends the harness — `launch-h2-instance.sh`)

`launch-h2-instance.sh N` (H2 counterpart of `launch-instance.sh`): 1 hardened pad,
per-instance TOML with the **`udp`** backend wired through the reflector, distinct-MAC
eeprom, own H2 overlay, sigshield + ptrace shim.

| inst | name | eeprom (MAC) | overlay | net bind→remote | QMP |
|---|---|---|---|---|---|
| 1 (host)   | `h2sl-1` | `eeprom3` (`00:50:f2:c2:af:03`) | `ovl-h2-1.qcow2` | `:9975`→`:9985` | `xemu-qmp-3way-h2sl-1.sock` |
| 2 (client) | `h2sl-2` | `eeprom4` (`00:50:f2:c2:af:04`) | `ovl-h2-2.qcow2` | `:9976`→`:9986` | `xemu-qmp-3way-h2sl-2.sock` |

Hub: `udp_reflector.py --bind 9975,9976 --refl 9985,9986` (separate from the CE rig's
997x/998x). Both overlays back the same `hdd-ceprof.qcow2` — **this is the problem (below)**.

## What WORKS (verified with screenshots + packet capture)

- Both instances boot to the H2 title, drive through `SIGN IN→Don't Sign In →
  main menu → SYSTEM LINK → CHOOSE PROFILE (Halo0001) → AVAILABLE GAMES` browser.
- Host (`h2sl-1`) `Create New Game` → **PREGAME LOBBY, "Party is OPEN"** (Turf, team BRs).
- **The NIC networking is correct.** A logging pass-through of the reflected UDP
  (decoding the encapsulated Ethernet frames) shows a healthy bidirectional exchange:
  - **host `h2sl-1`** (src `00:50:f2:c2:af:03`) broadcasts **259-byte** IPv4 frames to
    `ff:ff:ff:ff:ff:ff` — the game **advertisement**.
  - **client `h2sl-2`** (src `00:50:f2:c2:af:04`) broadcasts **83-byte** IPv4 frames to
    `ff:ff:ff:ff:ff:ff` — the discovery **query**.
  - **Distinct MACs**, both broadcasting, and the reflector forwards each to the other
    (reflector stats climbed symmetrically, rx/tx ~[119,119]). So the host's advertisement
    **does reach** the client's guest NIC.

## What's BLOCKED (the wall — with specifics)

`h2sl-2`'s **AVAILABLE GAMES list stays empty** (only "Create New Game") even after a
full re-enter/rescan of the browser and >40 s of the host broadcasting. The advertisement
is delivered at the network layer but the **H2 client application rejects/ignores it**.

**Root cause (diagnosed): the two instances are the SAME console.** Both overlays back
`hdd-ceprof.qcow2`, so they share the **console name** and the **profile `Halo0001`**;
all eeproms also share the factory **serial `757934831097`** (only the MAC differs).
H2's System Link almost certainly filters games by **console/profile identity** (serial /
XUID / console-name embedded in the 259-byte advertisement) and drops one that matches
the client's own — i.e. "that's my own game, hide it." **CE tolerated this exact clone
setup** (same base, same serial, distinct MAC) and linked fine on 2026-06-30, which is
why this is H2-specific, not a reflector/NIC problem.

**This is NOT a "xemu can't bridge two instances" wall** — xemu's `udp` backend + the
reflector bridge Halo system-link discovery correctly (proven for CE, and here the H2
frames demonstrably cross). The remaining blocker is H2 requiring two **distinct console
identities**.

## Fix direction (next step — differentiate the console identity)

Cheapest → most thorough (each needs a relaunch + re-drive to re-test):
1. **Distinct profile per instance** — create `Halo0002` on `h2sl-2` (ENTER-NAME flow) so
   the joining XUID differs. Quick test of the profile-XUID hypothesis.
2. **Distinct console name per overlay** — stamp a different `E:\UDATA\…\.XBN` name into
   each overlay (the M26 `internal/podman/console_name.go` / `fatx_console_name.py` path).
3. **Distinct eeprom serial** — patch `0x34` (12 ASCII) + recompute the Xbox factory
   checksum at `0x30`, one eeprom per instance.
4. **Distinct base disks** — back each overlay with a genuinely different H2 console image
   (e.g. two of the fleet `hdd2..5` if they are distinct consoles) instead of one shared base.

Recommend trying (1) or (2) first; they're the likely identity keys and don't need the
eeprom checksum. Evidence for this doc: live packet capture of the reflected frames
(headers above); host/client screenshots in `/tmp/xemu-sl/` this session.

## Console-nickname RE (2026-07-02) + confirmed filter key

**The System Link filter key is the EEPROM SERIAL — verified empirically:** changing ONLY
the serial (`757934831097`→`757934830002`, distinct MAC already present) made the client list
+ join the host's game. Console name and profile were NOT changed at that point. So the real
key is the serial, not the console name.

**Where the console nickname actually lives (corrects the "E:\NICKNAME" guess):**
`E:\UDATA\NICKNAME.XBN` — an XBN binary (`04 00 'SM'` header + UTF-16LE NUL-terminated name,
≤15 chars; the format `internal/podman/console_name.go` already writes). Software reads it as
`\Device\Harddisk0\Partition1\UDATA\NICKNAME.XBN`; the UnleashX skin shows it via `$NickName$`.
It is NOT in `cerbios.ini` or the eeprom. On the shared `hdd-ceprof` base the file is **absent
(no nickname set)** — so a shared nickname couldn't have been the filter key. RE method: dumped
the base with `qemu-img convert -O raw`, string-scanned the FATX partitions (C `0x8CA80000`,
E `0xABE80000`). The two H2 *players* already read as distinct names (Halo0001 vs Snake), which
is enough for unambiguous screenshots; distinct per-instance nicknames are cosmetic here.
