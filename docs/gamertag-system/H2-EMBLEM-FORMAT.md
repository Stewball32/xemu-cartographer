# Halo 2 emblem / appearance format (`profile` 0x118 block)

Reverse-engineered map of the Halo 2 player-profile **appearance + emblem** block,
plus the full value sets and the in-game composition model. This is the source of
truth behind `internal/halosave/h2profile.go` (`H2ProfileFields`) and
`sveltekit/src/lib/utils/emblem.ts`.

Status: **emblem/colour bytes `0x118..0x11F` are CONFIRMED** (decode cleanly against
the in-game `s_player_profile` struct and verified byte-for-byte against two real
captured profiles — see `internal/halosave/h2emblem_test.go`). The controller bytes
(`0x12B`/`0x12F`) and the `emblem_flags` semantics remain provisional.

## The appearance block

The H2 `profile` save is 500 bytes (`0x1F4`); the appearance/emblem block is the
24 bytes at `0x118..0x12F`, followed by a 20-byte HMAC digest at `0x1E0` (see
`docs/lan-hub/reference/FORMATS.md` §5 + `digest.go`). Each appearance field is a
single byte:

| Offset | Key (`H2ProfileFields`) | Meaning | Range | Enum |
| ------ | ----------------------- | ------- | ----- | ---- |
| `0x118` | `armor_primary`      | Armor primary colour      | 0–17 | `e_player_color` |
| `0x119` | `armor_secondary`    | Armor secondary colour    | 0–17 | `e_player_color` |
| `0x11A` | `emblem_primary`     | Emblem primary colour (tertiary)   | 0–17 | `e_player_color` |
| `0x11B` | `emblem_secondary`   | Emblem secondary colour (quaternary) | 0–17 | `e_player_color` |
| `0x11C` | `character_type`     | Player character model    | 0–3  | `e_character_type` |
| `0x11D` | `emblem_foreground`  | Emblem foreground symbol  | 0–63 | `e_emblem_foreground` |
| `0x11E` | `emblem_background`  | Emblem background shape   | 0–31 | `e_emblem_background` |
| `0x11F` | `emblem_flags`       | Emblem flags (provisional; 0 in all samples) | — | — |
| `0x12B` | `ctrl_a`             | Controller setting (provisional) | — | — |
| `0x12F` | `ctrl_b`             | Controller setting (provisional) | — | — |

This is exactly the in-RAM `s_player_profile` struct (Pack=1): `primary_color`,
`secondary_color`, `tertiary_color`, `quaternary_color`, `player_character_type`,
then `s_emblem_info { foreground, background, flags }`.

## Composition model

An H2 emblem is a square. The **background plate** is drawn in the two **armor**
colours (`armor_primary` + `armor_secondary`); the **foreground symbol** is drawn
inset on top in the two **emblem** colours (`emblem_primary`/`tertiary` +
`emblem_secondary`/`quaternary`). 18 colours × 4 slots × 64 foregrounds × 32
backgrounds. (Matches the predecessor's emblem renderer, which passed
`P, S, EP, ES, EF, EB` to a compositor.)

## Value sets (index = in-game enum order — authoritative)

### `e_player_color` (18)

`0 white, 1 steel, 2 red, 3 orange, 4 gold, 5 olive, 6 green, 7 sage, 8 cyan,
9 teal, 10 cobalt, 11 blue, 12 violet, 13 purple, 14 pink, 15 crimson, 16 brown,
17 tan`

### `e_character_type` (4)

`0 masterchief, 1 dervish, 2 spartan, 3 elite`

### `e_emblem_foreground` (64)

`0 seventh_column, 1 bullseye, 2 vortex, 3 halt, 4 spartan, 5 da_bomb, 6 trinity,
7 delta, 8 rampancy, 9 sergeant, 10 phoenix, 11 champion, 12 jolly_roger,
13 marathon, 14 cube, 15 radioactive, 16 smiley, 17 frowney, 18 spearhead, 19 sol,
20 waypoint, 21 yin_yang, 22 helmet, 23 triad, 24 grunt_symbol, 25 cleave,
26 thor, 27 skull_king, 28 triplicate, 29 subnova, 30 flaming_ninja,
31 double_crescent, 32 spades, 33 clubs, 34 diamonds, 35 hearts, 36 wasp,
37 mark_of_shame, 38 snake, 39 hawk, 40 lips, 41 capsule, 42 cancel, 43 gas_mask,
44 grenade, 45 santa, 46 race, 47 valkyrie, 48 drone, 49 grunt, 50 grunt_head,
51 brute_head, 52 runes, 53 trident, 54–63 number0…number9`

### `e_emblem_background` (32)

`0 solid, 1 vertical_split, 2 horizontal_split1, 3 horizontal_split2,
4 vertical_gradient, 5 horizontal_gradient, 6 triple_column, 7 triple_row,
8 quadrants1, 9 quadrants2, 10 diagonal_slice, 11 cleft, 12 x1, 13 x2, 14 dircle
(circle), 15 diamond, 16 cross, 17 square, 18 dual_half_circle, 19 triangle,
20 diagonal_quadrant, 21 three_quarters, 22 quarter, 23 four_rows1, 24 four_rows2,
25 split_circle, 26 one_third, 27 two_thirds, 28 upper_field, 29 top_and_bottom,
30 center_stripe, 31 left_and_right`

## Verification

Decoded from the two real captured profiles in `internal/halosave/testdata/h2/`:

| Sample | name | armor (P/S) | emblem (P/S) | character | foreground | background |
| ------ | ---- | ----------- | ------------ | --------- | ---------- | ---------- |
| `profile_587C76321326.bin` | Halo0001 | purple(13)/blue(11) | pink(14)/pink(14) | masterchief(0) | thor(26) | quarter(22) |
| `profile_E4CADA6B1E65.bin` | Stew | cobalt(10)/blue(11) | white(0)/white(0) | masterchief(0) | jolly_roger(12) | horizontal_split2(3) |

`TestH2EmblemOffsetsFromRealProfiles` asserts these and guards against drift.

## Colour palette (hex)

`emblem.ts` carries hex for both palettes, derived from the app's `--color-armor-*`
oklch tokens (tuned to in-game biped tints). The H2 palette uses `e_player_color`
order; the CE palette uses the c20.reclaimers.net order (which matches the CE
`blam.sav` `0x18` colour enum — Red=2 / Blue=3 confirmed against real saves).

## Halo: CE armor colour (cross-reference)

CE has no emblem. Its single armor colour is a `u32` at `blam.sav` `0x18`
(`ceprofile.go`), enum in c20 order. Only White(0)/Red(2)/Blue(3) are confirmed
in-game; the remaining names (steel/gray, green, yellow, …) come from the c20
reference table and are unconfirmed in-game.

## Still needs an in-game (xemu) capture

The offset map and value sets above are complete for building a profile, but a few
points are confirmed only from docs/struct, not from setting them in-game and
reading the bytes back:

1. **`emblem_flags` (`0x11F`)** — purpose unknown (0 in every sample). Set various
   emblems in-game and diff to see what toggles it (mirror/rotation? unlock state?).
2. **Controller bytes (`0x12B`/`0x12F`)** — still provisional labels.
3. **Round-trip acceptance** — confirm a generated profile with a chosen emblem is
   accepted by H2 on xemu and renders the intended emblem (sign path is already
   byte-verified; this validates the *appearance* bytes end-to-end in-game).
4. **CE colour carousel** — enumerate the full CE `0x18` index→name table in-game
   (only White/Red/Blue confirmed today).

These require Stewart's xemu session (a visible window — set the emblem in the H2
menu, then read the save). Everything else was done from docs + offline diffing.
