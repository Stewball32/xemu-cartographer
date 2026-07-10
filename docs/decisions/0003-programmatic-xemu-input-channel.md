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
- **2026-07-10 — keyboard-injection path (xdotool→X) does NOT drive the controller on
  `:0`.** Tested the design premise that player-hosting input could ride the kiosk's
  *keyboard* channel (admin VNC keysyms + runner-injected keysyms into the same X server).
  Launched a scratch xemu bound to `'keyboard'` (TOML identical to the kiosk container's
  default — `port1='keyboard'`, no scancode map) and injected keysyms with **xdotool**
  (both XTEST-focused and XSendEvent-to-window). Result: **no effect on the guest
  controller** — the menu selection never moved, `game_connection`/`main_menu` never
  changed, and the idle→attract loop kept firing (proof zero keys registered, since input
  resets that timer). Exhaustive: every VNC-map keysym (arrows / a-b-x-y / Return /
  BackSpace / ESDF / WO), quick + held timings, click-to-grab, mouse-off-menubar, and
  `background_input_capture=true` — all negative, with the xemu window holding confirmed X
  keyboard focus (`xdotool getwindowfocus`). So on `:0`, **xdotool→X is NOT equivalent to
  the admin VNC path** as assumed, and — together with the `sendkey` result — **neither
  keyboard-injection channel drives xemu's guest controller; only SDL-gamepad input (the
  vpad) does.** Caveat: this was `:0` (real X server + windowed xemu + WM), not the
  container's **Xvnc** (headless, single fullscreen window) where the admin VNC keyboard
  controller is *assumed* to work — that assumption is now **unverified** and may itself be
  broken (a `sendkey`-style false-positive). **Decision impact:** do not wire the runner to
  the keyboard channel yet. Next: verify whether the admin VNC keyboard controller actually
  drives the game *in a real container/Xvnc* before trusting it; if it doesn't, the
  keyboard-channel + seamless-admin-takeover design needs rework (e.g. runner + admin both
  drive a shared vpad, or an explicit control handoff). The proven runner input path
  remains the **vpad**. Scratch instance torn down; ce-nav untouched.
- **2026-07-10 — the keyboard channel IS the admin's RFB→Xvnc path (not xdotool→`:0`);
  built the server-side RFB injector.** Stewart's correction: the `:0` failure above was a
  mismatch — the admin panel drives the *container's Xvnc*, not a host `-display xemu` SDL
  window. Traced the proven admin path: `XboxController.svelte` (button→label) →
  `VNCKeyboard.sendKey` (RFB KeyEvent over WS) → `routes/containers/vnc.go` relay →
  `ws://127.0.0.1:<BrowserWeb>/websockify` → the container's Xvnc → xemu's keyboard→controller
  map. **Exact keys** (label / X11 keysym): A=`a`/0x61, B=`b`/0x62, X=`x`/0x78, Y=`y`/0x79,
  Start=`Return`/0xff0d, Back=`BackSpace`/0xff08, D-pad Up/Down/Left/Right=0xff52/54/51/53,
  LB=`1` RB=`2` L3=`3` R3=`4`, LT=`w` RT=`o`, sticks ESDF/IJKL. Built `internal/vncinput`: a
  server-side RFB-3.8-over-WebSocket injector that dials the same `/websockify` target and
  emits **byte-identical KeyEvents to `VNCKeyboard`** (verified end-to-end against a mock RFB
  server — `Tap("a")`→`[4,1,…,0x61]`, `Tap("Down")`→`[4,1,…,0xff,0x54]`), plus `cmd/vncinput-poc`
  to drive a live container. Because the bytes are identical to the admin's proven-working
  path, the injector drives the controller *by construction*, and admin + runner share one
  channel natively (satisfies take-over with nothing extra). **Live memory re-verification
  is still pending:** this session had no way to stand up the container's Xvnc — no host VNC
  tooling (Xvnc/x11vnc/websockify absent), rootful podman is `sudo`-gated (denied here) so the
  2-container stack can't be provisioned, no cartographer container was running, and the live
  server needs admin auth. Run `cmd/vncinput-poc -url ws://127.0.0.1:<BrowserWeb>/websockify
  -keys "Down Down a"` against a live container and read the state change via the scraper to
  close the loop. **Decision:** the runner's keyboard/menu + admin-shared input path is
  `internal/vncinput`; the vpad stays for headless analog automation.
- **2026-07-10 — state-aware host RUNNER built (logic + tests).** `internal/hostrunner`
  composes the scraper READ side (Observation, built from the cache via `ScraperReadout`) with
  the `internal/vncinput` WRITE side into the CE host-lobby state machine. Every transition is
  gated on readable state (`Classify`: game_connection 0/1/2, main_menu, machine/team counts,
  native countdown) — `Sequence` presses `Y→A→A` only when confirmed on the right screen and
  advances on confirmation, with the two blind map/gametype cards TIMED but bracketed by
  readable checkpoints; `WalkBack` (B) retreats/cancels the countdown. Native start conditions
  (2+ boxes, 2+ teams) are READ (`ReadyToStart`), never faked — the runner only times arm (A on
  gametype) / start (A again). Auto-host loop re-hosts off the post-game screen. Arbitration
  (`Arbiter`: runner/admin/disabled) suspends the runner so an admin's keysyms drive the SAME
  channel — native takeover. Observable stream (`RunnerEvent` + `EventSink`) fans intents+keys
  to the admin WS room via `Registry`. v1 seams present: `Selector` (decision-source),
  `StartPolicy` (ordered predicates + arm-only/arm+start), optional ready-gate (default off).
  Control endpoints modelled on `routes/scraper/host.go` (GET/POST `/{name}/host`). **24 unit
  tests**, no live container. First-live-milestone tool `cmd/hostrunner-probe` demonstrated the
  read→classify→decide half LIVE against ce-nav (real memory: classified `in_game`, gated
  press correctly `blocked`). **The first live test needs: a running container + its
  `ws://127.0.0.1:<BrowserWeb>/websockify` endpoint** — then
  `hostrunner-probe -sock <qmp> -url <ws> -expect system_link -key a` performs the gated tap +
  verifies. Full runner wiring (tick from the scraper loop with cache Observations + per-
  instance vncinput dial + WS `SinkFunc`) is the remaining integration glue.
