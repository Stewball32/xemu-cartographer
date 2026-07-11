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
- **2026-07-10 — live container confirmations (provisioned via the app's own Manager).**
  Brought up a throwaway `livetest` pair with `cmd/podman-harness` (drives `internal/podman`
  Manager.Create/Start over passwordless `sudo podman`; BrowserWeb=3603, QMP at
  `containers/xemu/qmp/livetest.sock`).
  - **2b Firefox cert-fix — PASS.** `certutil -L` shows `xemu-cartographer dev CA` (`C,,`) in
    the profile's cert9.db; the enterprise policy installs `/xemu-cert/ca.pem`; xemu HTTPS:3601
    serves the SAN-pinned leaf (`CN=localhost`, issuer `CN=xemu-cartographer dev CA`); a fresh
    Firefox kiosk reaches the xemu Selkies view with **no "risky connection" interstitial**
    (screenshot) and **zero SEC_ERROR** in the logs.
  - **2a vncinput RFB — PARTIAL.** `cmd/vncinput-poc` completed the RFB-3.8 handshake against
    the live container websockify (`ws://127.0.0.1:3603/websockify`) and delivered keys; xemu
    reacted (process exited), so input DOES traverse the real path (RFB → browser Xvnc →
    Firefox → Selkies → xemu). No clean guest-memory delta because Halo never booted (below).
  - **2c runner gated press — BLOCKED** by the same boot issue (runner logic already proven
    live on ce-nav).
  - **Blocker: the container's Xbox guest hangs in the 2BL/Cerbios boot loop** — EIP loops in
    `0x1b–0x20xxxx`, the kernel's `0x80000000` mapping never appears. Ruled out: HDD/overlay
    (verified 128 GiB / 8.42 GiB Halo install, backing resolves), net (disabled cleanly), GL
    (hang is pre-video). Likely an **eeprom ↔ `_default.qcow2` HDD-lock pairing gap** the
    provisioning doesn't resolve (neither the image's auto-eeprom nor the host `eeprom-ceprof.bin`
    boots it). Also **image drift**: the current `lscr.io/linuxserver/xemu` uses **openbox**, not
    the **labwc** autostart `Manager.Create` writes — so xemu's `-qmp` (and the `04-patch-startwm`
    GPU-fallback) never apply (I injected `-qmp` into the openbox autostart by hand).
  - **Bug found + FIXED:** with the default `PodmanCmd="sudo -n podman --runtime=crun"`,
    `runSudo` failed to strip `podman` (it only dropped a *trailing* `podman`), so
    `removeContainerFiles`/`CleanupOrphans` ran `sudo podman rm -rf` (rejected) and orphaned every
    removed instance's bind-mount files. Fixed with `sudoPrefix` (+ unit tests). Instance torn
    down; box left clean; ce-nav untouched.
- **2026-07-10 (cont.) — both blockers FIXED; 2a + 2c now PASS on a freshly-provisioned instance.**
  - **`-qmp` autostart (CRITICAL) — FIXED + CONFIRMED.** `Manager.Create` now writes the xemu
    launch (carrying `-qmp unix:/qmp/<name>.sock`) into **both** the openbox (X11) *and* labwc
    (Wayland) autostart paths (`writeXemuAutostart`), so whichever WM the current image runs, the
    flag fires — robust to the openbox↔labwc drift instead of hard-pinning either. Added a loud
    provisioning assert: `Start` calls `WaitForQMP` (75 s) and errors if the bind-mounted socket
    never appears. Verified live: the freshly-provisioned container's QMP socket came up and the
    scraper's read path read live CE globals through it. (`TestXemuAutostartCarriesQMP` guards both.)
  - **Halo boot hang — ROOT-CAUSED + FIXED.** The cause was **not** GL or a code bug: a real
    (HDD-**locked**) Halo disk only unlocks in 2BL/Cerbios with its **paired eeprom (HDDKey)**, and
    the provisioning seeded **no eeprom** at all (neither `internal/podman` nor the init scripts) —
    so xemu self-generated a **random** eeprom whose key never matches, and the guest loops in
    2BL forever (the exact reported symptom). Proven by A/B: `hdd-ceprof` + a random eeprom hangs;
    `hdd-ceprof` + its paired `eeprom-ceprof.bin` boots to the CE main menu. **Fix:** new
    `Config.RootEeprom` (env `CONTAINERS_ROOT_EEPROM`) — `seedEeprom` copies the root's paired
    eeprom into `<config>/.local/share/xemu/xemu/eeprom.bin` at create time (idempotent; abs or
    relative to SharedDir; no-op when unset for an unlocked root). Also added `Config.DVDPath`
    (env `CONTAINERS_DVD_PATH`) to bind-mount a Halo ISO at `/game.iso` for disc boot. **The
    shipped `_default.qcow2` still needs a matching eeprom (or replacement with a known-good
    bootable Halo disk) — that pairing is the operator's to supply via `CONTAINERS_ROOT_EEPROM`.**
    Confirmed end-to-end: `Manager.Create` with `RootEeprom` set (zero manual steps) boots CE to
    the main menu — kernel `0x80000000` maps (`deadbeef`), `main_menu=1`, `game_connection=0`,
    full-screen HALO menu screenshot. (`TestSeedEeprom` guards seed/idempotency/path-resolution.)
  - **2a vncinput RFB memory-delta — PASS.** With the Selkies canvas focused (one RFB
    PointerEvent) the injector's KeyEvents drive the live game: screenshot-verified menu
    navigation (Down×3 → GAME DEMOS, Up → MULTIPLAYER, A → submenu, team toggle Red→Blue) and a
    clean **fixed-global memory delta — `game_connection` 0→2** (menu → hosting) read back over
    QMP as the runner navigated into a split-screen host lobby. **Focus gotcha:** Selkies only
    forwards keys when its canvas has DOM focus; a pointer click focuses it and focus persists
    across RFB reconnects. The `vncinput` injector is KeyEvent-only (no PointerEvent) — a runner
    that starts cold may need a focus-click first (follow-up).
  - **2c hostrunner gated press — PASS (end-to-end).** Added a **QMP-memsave read mode** to
    `cmd/hostrunner-probe` (`-readvia qmp -memsave-dir /qmp`): memsave needs no ptrace, so the
    probe reads a **rootless** container's memory where the `/proc/<pid>/mem` path is blocked
    (YAMA `ptrace_scope=1` denies a uid-1000 non-ancestor even at the same uid; production reads
    `/proc` as root). Demonstrated the full read→classify→gate→act→verify: the gate **blocks** on
    a wrong `-expect` ("expected main_menu, observed hosting — not pressing") and **taps** on the
    right one, firing a single A press through `internal/vncinput`, with the verify confirming
    `game_connection` **0→2** (main_menu → hosting). livetest torn down; test-only shared disks
    removed; `_default.qcow2` perms restored; box left clean.
- **2026-07-10 (correction) — the eeprom/HDD-lock diagnosis above was WRONG; `_default.qcow2`
  boots fine. Firmware also ruled out. The sole real bug was the `-qmp` autostart drift.**
  Prompted by Stewart to verify the firmware/TOML paths, I ran the controlled test the earlier
  A/B skipped (it changed disk + eeprom + net + DVD at once, and its "working boot" used
  `hdd-ceprof`, never `_default.qcow2` itself). Findings, all direct-observed:
  - **Firmware is correct + identical to the working host rig.** The base template + every
    per-instance TOML point at `/shared/bios/mcpx_1.0.bin` + `/shared/bios/Cerbios.bin`; those
    bind-mounts resolve and are **byte-identical** to `~/.local/share/xemu/xemu/{mcpx_1.0.bin,
    Cerbios.bin}` (md5 `d49c52a4102f` / `1255c9d4fe90`). Corroborated: the container booted
    `hdd-ceprof` to Halo using this exact firmware — a wrong BIOS/MCPX can't boot any disk.
    **BIOS/bootrom is NOT the cause.**
  - **`_default.qcow2` BOOTS — the disk is good and the eeprom pairing is NOT needed.** A fresh
    `Manager.Create` on `_default.qcow2` (128 GiB / 8.42 GiB used, standalone, its own HDDKey)
    with a **RANDOM** eeprom booted straight to a working **UnleashX dashboard** (FATX E:/F:
    mounted, a "Play Halo" entry) and **launched Halo CE** ("Play Halo" → the CE Spartans
    loading screen; kernel `0x80000000` maps throughout). This holds with **net on or off** —
    net-enabled only adds a *non-fatal* xemu pcap dialog ("Operation not permitted", a
    PUID=1000/NET_ADMIN harness artifact; prod runs root) shown *over* the already-booted guest.
    So **eeprom, firmware, net, and HDD-lock are each individually falsified** as the boot cause:
    with a modded BIOS (Cerbios) the ATA HDD lock is ignored, so any eeprom boots any disk.
  - **Conclusion:** the prior "2BL hang / kernel never maps" was a **misdiagnosis** — almost
    certainly a downstream symptom of the `-qmp` autostart drift (labwc autostart written while
    the image runs openbox → our launch/`-qmp`/config never applied, so the guest state couldn't
    be observed and manual reads misread it). The one real, confirmed fix is the **`-qmp`
    autostart written to both openbox+labwc + the `WaitForQMP` assert**. With that in place a
    fresh `_default.qcow2` instance boots to the dashboard and launches Halo unaided.
  - **Reverted:** the `RootEeprom`/`seedEeprom`/`CONTAINERS_ROOT_EEPROM` provisioning + its test
    (fixed a non-problem; Cerbios ignores the lock). **Kept:** the `-qmp` autostart fix +
    `WaitForQMP` (the real fix); `DVDPath`/`CONTAINERS_DVD_PATH` (an independent, generically-
    useful knob to boot Halo from an ISO); and `hostrunner-probe -readvia qmp` (rootless reads).
    deftst torn down; `_default.qcow2` perms restored; box left clean.
- **2026-07-10 — INTEGRATION GLUE landed: the runner is now wired live into the server.** The
  state-aware runner is no longer a standalone component — it's driven per instance from the
  scraper loop and exposes a player-scoped API. What shipped:
  - **Tick from the scraper loop.** `manager.runner.tickHost` (in `manager/hostrunner.go`) builds a
    `hostrunner.ScraperReadout` from LOOP-OWNED state only — the reader's `LastStateInputs`
    (`main_menu` + the new `game_connection`, added to the CE reader's `ReadGameState`) and the
    loop's working `GameData` copy — then calls `Runner.Tick` from `runReady`/`runLive`. So the
    runner never touches a GameReader off-goroutine; `game_connection`'s low-GVA translation rides
    the existing bind-time `Init`/`RefreshLowHVA` discipline (re-Init on every XBE swap via
    `runIdle`). Throttled to ~2.5 Hz (`hostTickMinInterval`) so the 30 Hz Live poll doesn't flood.
  - **vncinput attaches per instance.** `internal/vncinput.Pump` — an async command queue owning one
    `*Injector` on its own goroutine (lazy dial + reconnect + focus-click on connect via the new
    `SendPointer`/`FocusClick` RFB PointerEvent, addressing the Selkies DOM-focus gotcha). The
    Manager dials it to the container's `ws://127.0.0.1:<BrowserWeb>/websockify` (resolved through
    the podman manager) on `Start` and Closes it on `Stop`, tied to discovery add/remove. Keeps the
    Injector single-goroutine while never blocking the scraper loop on RFB writes.
  - **WS stream → admin room.** A `hostrunner.SinkFunc` (built in `cmd/server/hostrunner.go`)
    marshals each `RunnerEvent` into a `"host_runner"`-typed `websocket.Message` and
    `svc.WS.SendToRoomRaw("admin", …)` — player intents, emitted keys, native box/team counts.
  - **Arbitration + player API.** `scraperroutes.SetHostControl` now wired (`GET`/`POST
    /api/admin/scraper/{name}/host` = run/override/disable). New player-scoped `/api/play/*` group
    (RequireAuth, NOT admin) resolves the caller's container via `gamertags.SanitizedForUser` +
    `scraperiface.MatchContainer(Membership())` (admin `?container=` override): `current`, `options`
    (CE map/gametype catalog + selection), `selection`, `ready`/`unready` (arm+start toggle),
    `request`/`teardown`. Player actions touch ONLY player-intent (ready/selection) and are refused
    (409) when the runner isn't runner-driven — an admin takeover always wins. Gated behind
    `HOSTRUNNER_ENABLED` (default off); the admin kiosk + VNC keyboard path is untouched.
  - **Pre-existing router bug fixed.** The `host.go` `POST /{name}/host` collided with the earlier
    `POST /scraper/stop/{name}` in Go's ServeMux (both match `/stop/host` → panic at registration),
    so the full server had been un-bootable since the runner commit (prior live tests used probes,
    not `serve`). Renamed the scraper stop route to the consistent `POST /{name}/stop` (no frontend
    caller; e2e mocks already expected that form).
  - **Validation.** Container-free logic is unit-tested (pump dial/reconnect/queue, readout builder
    + team/state-input helpers, runner ready/selection overrides, play resolve/catalog; race-clean).
    Live process-level smoke: the server now BOOTS with all routes mounted, and authed requests
    exercise the whole auth→resolve→status path (idle `current`, the 13-map/7-gametype catalog,
    404 on unmatched control, host-control `Status`, arbitration validation). The actual
    container-drive step (runner ticks → presses → guest state change) needs `sudo podman` +
    root memory reads, which this session's sandbox blocked; those primitives were already proven
    live on this branch (2a/2c above).
