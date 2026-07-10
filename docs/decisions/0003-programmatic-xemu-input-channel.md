# ADR-0003 — Programmatic xemu input: QMP `sendkey` vs. uinput virtual pad

> **Status:** Accepted
> **Date:** 2026-07-10

## Context

The scraper reads live game state every tick but has **no way to send input** into
xemu — all input today is human-driven over VNC (the kiosk / `VNCKeyboard` path).
The player self-service hosting feature needs a **headless, backend-owned input
channel** straight into xemu, separate from the admin kiosk / VNC relay.

The first candidate was QEMU's HMP **`sendkey`**, reachable over the QMP socket the
scraper already holds (`internal/xemu/qmp.go` `hmp()`). The theory: xemu maps the
Xbox pad to keyboard keys, so a `sendkey` naming the same keys the frontend VNC map
uses (`sveltekit/src/lib/utils/vnc-keyboard.ts`) would drive the controller.

We built the primitive (`internal/xemu` `SendKey` / `SendChord` / `…Hold`, translating
the VNC labels to QEMU keycodes) and tested it live against the running `ce-nav`
instance (xemu 0.8.136, hosting a Blood Gulch Slayer, in-game with a live biped).

**Empirical result** (`cmd/inputpoc`, read → act → verify):

- State gate read `in_game`, located the local player ("Stew") biped.
- `SendKeyHold("e", 900ms)` (left-stick-forward) issued cleanly (HMP returned success)
  → **biped displacement 0.0000; no observed effect.**
- Positive control on the **same biped** via the padpool virtual-pad FIFO (`move 1`)
  → **biped moved 1.74 world units** — the observable and verify path are sound; only
  the `sendkey` link did nothing.

**Why.** Two independent reasons: (1) `ce-nav`'s ports are bound to virtual **uinput
gamepads**, not `'keyboard'`, so keyboard events can't reach the pad. (2) Structurally,
xemu's keyboard→controller mapping is the **SDL scancode** path
(`keyboard_controller_scancode_map`) — the same path the kiosk's VNC keyboard feeds
through the container's X server. QMP `sendkey` delivers via QEMU's `qemu_input` layer
to the emulated i8042 / USB keyboard, a **different channel** the Xbox game (and xemu's
controller emulation) never reads. So `sendkey` is very unlikely to drive the
controller even on a keyboard-bound instance.

## Decision

- **QMP `sendkey` is NOT the input path for Xbox controller input.** Do not build
  player-hosting input on it.
- **Keep** the `internal/xemu` `SendKey` / `SendChord` primitive: it is minimal,
  unit-tested, and is the correct home for guest-keyboard / xemu-monitor keystrokes,
  and for the day a keyboard-bound instance is proven drivable. Its doc comment carries
  the routing caveat.
- **The input keystone for player-hosting is the uinput virtual pad** — the mechanism
  `scripts/runtime/padpool.py` already uses and that this POC confirmed live. The next
  step is a backend-owned Go primitive that creates a virtual Xbox-360 pad
  (`/dev/uinput`) and writes HID reports, with xemu launched bound to that pad's SDL
  GUID (mirroring the podman/TOML binding the offset rig uses).

## Consequences

- **Positive:** the build plan avoids a dead end; the read → act → verify loop is proven
  end-to-end (`cmd/inputpoc`); the `sendkey` primitive is retained where it's genuinely
  useful; the winning mechanism is identified and already demonstrated on live hardware.
- **Negative / cost:** the working path (uinput pad) is more involved than a QMP call —
  it needs device access, a HID report format, and per-instance launch-time GUID binding,
  and it lives outside the QMP socket the scraper already owns. Porting `padpool.py`
  behaviour into the server is follow-up work (provisioning + the state-aware runner).
- **Open verification:** we did not test `sendkey` against a **keyboard-bound** instance
  (the player containers, like the kiosk, bind `'keyboard'`). The architectural reasoning
  says it still won't reach the controller, but a definitive keyboard-bound check is the
  one remaining way to fully close the door on `sendkey` before committing.

## Log

- **2026-07-10 — uinput virtual pad BUILT + PROVEN live.** Ported `padpool.py` into a
  backend-owned Go primitive (`internal/vpad`) that creates a uinput Xbox-360 pad and emits
  button/D-pad/stick/trigger events, with a `cmd/vpad` daemon holding the device + serving a
  drive FIFO. The derived SDL GUID reproduces ce-nav's TOML binding exactly
  (`PredictGUID` / `NameCRC16`, unit-tested). Proof: launched a scratch xemu bound to the Go
  pad's GUID (`0x0701`, distinct from ce-nav), drove it through the CE menus into a Campaign
  level entirely with the Go pad (menu selection moved; `game_connection` 0→2 then
  `main_menu` 1→0 in guest memory), then in gameplay the same read→act→verify loop
  (`cmd/inputpoc -control-fifo`) walked the biped **+1.71 world units** (`disp=1.7141`) —
  matching padpool's 1.74wu — while QMP `sendkey` moved it 0.0000 in the same run. The Go
  virtual pad drives the game; `sendkey` does not. Scratch instance torn down; ce-nav
  untouched.
