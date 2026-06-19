# Milestone 24 — Remote game setup (controller-free lobby write)

> **Status:** Planned
> **Depends on:** M01 (xemu memory bridge), M02 (Halo: CE reader + offset table), M03 (containers), M09 (per-instance container access), M19 (offset validation — hardens the write path). **Strongly recommended to land _after_ M19.**

> Today every memory access in this project is **read-only** (`internal/xemu/mem.go` is `Pread`-only over `/proc/<pid>/mem`). M24 introduces the first **memory writes**: an operator opens a web page for a running xemu instance, the page lists the Halo titles installed on that instance's Xbox HDD, they pick a title → see its maps + multiplayer gametypes → click one → the project **writes** the selection into the instance's guest memory so the game drops into a configured **system-link lobby** with that map + gametype — **with no controller and no emulated controller input**. The hard parts are (a) doing writes safely in a read-only-by-design system and (b) reverse-engineering the menu/lobby/host state well enough to drive it by poking memory instead of pressing buttons.

This milestone is **design-heavy reverse engineering**; the spec is doable now, but **build + verification require a live xemu running Halo: CE** (write-and-observe against a real lobby — there is no offline substitute).

## Scope

**In:**

- A guarded, bounded, fail-safe **write primitive** with dry-run/diff + rollback (24a).
- **HDD enumeration** of installed Halo titles + the loaded title's maps + multiplayer gametype variants (24b).
- The **memory map** of lobby/variant config, the menu/system-link state machine, and host state (24c — the core RE).
- The **remote-lobby web flow + write** that lands a running instance in a system-link lobby with a chosen map + gametype (24d).

**Out (this milestone):**

- Writing for any title other than **Halo: CE** (the offset table is CE-only; Halo 2 is M20 and would get its own write map).
- Emulated controller input / virtual gamepad (explicitly _not_ how this works — see 24c; we drive state directly).
- In-match writes (changing map/gametype mid-game, teleporting players, etc.) — M24 only sets up a lobby from menu/idle state. Anything beyond lobby setup is a follow-up and a much bigger safety surface.
- Multi-instance orchestration (auto-host a whole 4-pod match). M24 configures **one** instance; wiring N instances into one match is a follow-up.

## 24a. Guarded write primitive (the safety foundation)

The single most important and most dangerous piece. A wrong write doesn't return an error — it **corrupts guest state and crashes the game** (or worse, silently desyncs the lobby). Treat the write path like a loaded gun: default-closed, allowlisted, dry-run-first, reversible.

- **Primitive.** Add `WriteBytesAt(hva, data)` / `WriteBytes(gva, data)` to `internal/xemu/mem.go`, symmetric to the existing `ReadBytesAt` but via `syscall.Pwrite` on `/proc/<pid>/mem`. The mem fd must be opened **`O_RDWR`** (today it's read-only) — gate that behind a build/runtime flag so a read-only deployment never even holds a writable handle. Writing `/proc/<pid>/mem` needs the writer to be the same uid as xemu (or `CAP_SYS_PTRACE`); the scraper already runs as root alongside the root-in-container xemu (see CLAUDE.md "Containers"), so the permission exists — but make the writable-open **opt-in per instance** (a `writes_enabled` flag on the container record, default false).
- **Write-allowlist, not free-form.** Never expose "write arbitrary GVA". Expose a small set of **named, typed write ops** (e.g. `SetLobbyGametype(id)`, `SetLobbyMap(tag)`, `EnterSystemLinkHost()`), each bound to a specific (gva/deref-chain, size, encoder, validator). A request names an op + value; the layer resolves the address, validates the value against the op's allowed range, and writes only that span. Anything not in the allowlist is rejected. This is the M08-style "permission bundle" idea applied to memory.
- **Dry-run / diff mode (default).** Every write op supports `dryRun`: read the current bytes at the resolved target, render `{gva, before[], after[], decoded_before, decoded_after}`, and return it **without writing**. The web flow shows this diff and requires an explicit confirm before a real write. A diff that doesn't match the operator's expectation (e.g. `before` isn't a plausible lobby-state value) is the loudest signal that an offset drifted — abort.
- **Read-back verify.** After a real write, re-read the span and assert it equals what was written; on mismatch, log loud + attempt rollback. (Guest code may immediately overwrite a field — distinguish "write didn't take" from "game consumed it" per-op.)
- **Backup + rollback.** Capture the original bytes of every span a request touches _before_ writing; keep them for the request's lifetime so a failed multi-field sequence can be reverted. Persist a small audit row (`audit_log` per [ADR-0002](../decisions/0002-unified-audit-log-collection.md): `action="memory_write"`, payload = op + before/after + actor) so every write is attributable.
- **Atomicity for multi-field writes.** Setting up a lobby touches several fields (mode + map + gametype + host trigger). Between writes the guest is running and may observe a half-configured state. Use QMP `stop` (pause vCPU) around a write batch and `cont` after, so the game sees the whole batch at once. (QMP is already connected per instance — `internal/xemu`.) Keep the paused window minimal.
- **Validation via M19.** The M19 offset-validation sanity checks (base-HVA range, magic-value probes, plausibility bounds) are the precondition for a write: **re-validate the read of the target region immediately before writing**. If the region doesn't look like what we expect (offset drift, wrong title loaded, not at a menu), refuse. This is why M24 should follow M19.
- **Risk summary to put in front of the operator:** a bad write can crash the emulated game or hard-lock the lobby; mitigations are allowlist + dry-run-diff + value validation + read-back + rollback + QMP-pause + the per-instance `writes_enabled` opt-in. Document a "if the guest crashes, restart the container" recovery path (M03 already supports stop/start).

## 24b. HDD enumeration (read-side)

To list "what Halo games are installed" + "what maps/gametypes does the loaded one have", we need to see the Xbox HDD contents. The HDD is a **FATX**-formatted image (`containers/xemu/shared/hdds/*.qcow2`, see [03-setup-hdd.sh](../../containers/xemu/init/03-setup-hdd.sh)). Two complementary read paths:

- **(i) FATX parse, host-side (authoritative for "installed titles").** Read the qcow2 → FATX partition table → walk the standard Xbox partitions (`C:` system, `E:`/`F:` games + `TDATA`/`UDATA`). Installed titles live under their title-ID dirs; Halo's saved multiplayer **gametype variants** live in `UDATA/<titleID>/...` (player-saved profiles + variants). This needs a small read-only FATX reader (partition geometry + FATX dirent walk) over the qcow2 (decode qcow2 → raw, or mount read-only loopback host-side). **Read-only**, host-side, no guest interaction — low risk. Note: enumerating a _running_ instance's qcow2 while xemu has it open is a consistency concern — read the FAT/dir structures, accept eventual-consistency, or snapshot.
- **(ii) In-memory enumeration (authoritative for "what the loaded title exposes").** The loaded game already enumerates its maps + built-in gametypes (1–7, see `OffGVGametype`/`OffGEGGametype`) and the saved variants it has browsed. The maps available to a system-link host are the title's scenario/map list; built-in gametypes are the fixed 1–7; saved variants are what 24b(i) finds on the HDD. Reading the game's in-menu map/variant lists from memory is the lower-effort path for the _loaded_ title and reuses the M02 reader patterns.
- **Recommendation:** ship (ii) for the loaded title's maps + the fixed built-in gametypes first (it's read-only memory like the rest of the scraper), and add (i) FATX parsing for the cross-title "which Halo games are installed" list + saved-variant discovery as a second step. The web flow's "list installed Halo titles" needs (i); "pick this title's maps + gametypes" can lean on (ii) for the currently-loaded title.

## 24c. Advanced memory mapping (the reverse-engineering core)

This underpins 24d. We must map enough of three structures to set up a lobby by writing, **without emulated controller input** (we never touch the gamepad state — see below — we write the lobby/menu/host state the buttons would have produced). The existing offset table ([internal/scraper/haloce/offsets.go](../../internal/scraper/haloce/offsets.go)) already exposes the read-side anchors; the writes need a few more fields re-derived against the Xbox build.

### Structures to map

1. **Lobby / variant config** (selected map + gametype):
   - _Known (read side):_ `RefAddrGlobalVariant` (0x2F90A8) → global_variant container, gametype at `OffGVGametype` (+0x18); `AddrGameVariantGlobalPtr` (0x2FAB60); `AddrVariant` (0x2F90F4, u8 mode index); `AddrGlobalStageName` (0x2FAC20, host map/stage name string); `AddrGlobalScenarioPtr` (0x39BE5C, loaded scenario ptr); game-globals `OffGGMapLoaded` (+0x00) / precache (+0x04).
   - _Needs RE (write side):_ the **selected-but-not-yet-loaded map** field (the lobby's chosen scenario tag/index, distinct from the _loaded_ scenario the scraper reads today); the **saved-variant selection** (which on-HDD variant slot, vs. the built-in 1–7 gametype id); and the exact write encoding (does the host want the map tag, the scenario path string at `AddrGlobalStageName`, an index, or all three coherently?).

2. **Menu / system-link state machine** (how to "navigate" by writing):
   - _Known:_ `AddrGameConnection` (0x2E3684, u16: 0=menu/SP, 1=syslink, 2=hosting, 3=film) — the top-level mode; `AddrMainMenuActive` (0x2E4068, u8).
   - _Needs RE (the big one):_ the **menu screen / widget state stack** — which UI screen is active and the legal transitions (main menu → multiplayer → system link → create/host game → lobby). To drive the menus controller-free we either (a) write the screen-stack state directly to jump to the host-lobby screen, or (b) set the higher-level globals (`AddrGameConnection` → hosting, map/variant fields, host trigger) such that the engine assembles the lobby without walking the menu UI at all. Determining which approach the engine tolerates (does it require a valid menu-screen state, or will it honor the globals?) is the central RE question and the riskiest unknown.

3. **Host / session state** (open the system-link lobby):
   - _Known:_ `RefAddrNetworkGameServer` (0x2FBE40, network_game_server struct); `RefAddrNetworkGameClient` (0x2FB180, network_game_client).
   - _Needs RE:_ the **host/advertise start trigger** — the field/flag/function-state that makes the engine create + advertise a system-link session as host (transition `AddrGameConnection` 0→2 cleanly, populate network_game_server, begin broadcasting). Plus the lobby "ready/start countdown" fields if we later want to auto-start the match (out of scope for M24's "land in a lobby").

4. **Gamepad state — to AVOID, not write** (`RefAddrGamepadState` 0x276A5C, stride 0x28/player; `RefAddrGamepadStateAlt` 0x276AFC): documented here so the design is explicit — M24 does **not** synthesize button presses into the gamepad struct. That path (fake controller input) is fragile and timing-dependent; M24 writes the resulting lobby/menu/host state directly. Mapping the gamepad struct is only relevant as the thing we are deliberately not using.

### RE method (live, write-and-observe)

For each "needs RE" field: from a live Halo: CE instance at the main menu, snapshot the candidate region, then perform the action by hand (controller/keyboard into xemu) — enter system link, host a game, pick map X, pick gametype Y — diffing the region after each step to localize the field, exactly as the M02/M19 offset-derivation passes did (see the M19 Log's memory-probe methodology). Cross-reference `atlas/HaloCaster` for Halo-PC analogues but **re-verify every address against the Xbox build** (the M19 Log shows several PC-vs-Xbox offset traps). Then validate the write by poking the field and observing the menu/lobby react.

## 24d. Remote lobby setup (the web flow + the write)

The operator-facing feature, built on 24a–24c. Admin/operator-gated (reuse M08 roles + M09's per-container access; likely a new `remote_setup` permission, and only on instances with `writes_enabled`).

- **UI flow:** a page per instance (e.g. `/admin/pod/[name]/setup/` or a tab on the existing pod hub) → **list installed Halo titles** (24b-i) → pick one → **list its maps + multiplayer gametypes** (built-in 1–7 + saved variants, 24b) → operator picks map + gametype → **"Set up lobby"** → show the 24a **dry-run diff** of every field that will be written → confirm → execute the guarded write batch (QMP-pause → write mode + map + gametype + host trigger → cont → read-back verify) → poll the scraper's existing `host:<name>` state to confirm the instance is now in a system-link lobby with the chosen config.
- **Backend:** a new route group (e.g. `/api/admin/instances/{name}/setup/*`, RequireAuth + the remote-setup permission) exposing: `GET .../titles`, `GET .../titles/{id}/maps`, `GET .../titles/{id}/gametypes`, `POST .../lobby` (body `{title, map, gametype}`, `?dry_run=true|false`). The POST resolves to a 24a write batch via the named-op allowlist; dry-run returns the diff, real run executes + audits.
- **Feedback loop:** the existing scraper already reads `AddrGameConnection`, the variant, the stage name, and the network structs — surface those post-write so the UI can confirm "you're now hosting `<map>` / `<gametype>` on system link" using the read path we already trust. This closes the loop and is the in-product verification.

## Verification

**Spec-only deliverable now.** Build + verification need a live xemu running Halo: CE:

- 24a: dry-run diff matches a hand-made change; a real write of a benign field round-trips (read-back equals written); rollback restores; an out-of-allowlist or out-of-range write is refused.
- 24b: the installed-titles list matches what's on the HDD; the loaded title's maps + gametypes match the in-game menu.
- 24c: each "needs RE" field is localized by the diff method and a poke visibly moves the menu/lobby/host state.
- 24d (the milestone smoke test): from a fresh instance at the main menu, an operator clicks Title → Map → Gametype → Set up lobby → confirm; within a couple seconds the instance is **hosting a system-link lobby** with the chosen map + gametype, **no controller touched**, and a second idle instance on the same system-link network can see/join the advertised lobby. Confirm the scraper's `host:<name>` state reflects the chosen config. Then crash-test: a deliberately bad (allowlist-bypassed) write is shown to crash the guest, proving why the allowlist + dry-run exist, and `container restart` recovers.

## Log

_Append-only. Never edit past entries; add a new dated line._

- 2026-06-19: Spec drafted (design only; no implementation). First **write**-capable milestone in an otherwise read-only project — the 24a guarded-write primitive + 24c menu/lobby/host RE are the load-bearing risks. Build + verification deferred to a live xemu + Halo: CE session (write-and-observe). Recommended to land after M19 so the offset-validation sanity checks gate every write. Memory structures to map listed in 24c: lobby/variant config (selected map + gametype), menu/system-link state machine (screen stack + the host-lobby transition), host/session state (network_game_server + the advertise trigger), and — deliberately excluded — the gamepad state.
