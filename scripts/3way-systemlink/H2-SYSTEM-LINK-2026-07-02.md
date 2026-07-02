# Halo 2 two-instance System Link — networking SOLVED, app-level identity wall (2026-07-02)

Goal: get two xemu instances into a shared **Halo 2** System Link lobby. Result:
the **networking is working** (both instances discover-broadcast to each other over
the reflector), but the H2 client **does not list** the host's game — an H2
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
