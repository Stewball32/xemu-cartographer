# ADR-0004 — ISO-boot provisioning: attach a game XISO as the DVD, boot straight into it

> **Status:** Accepted
> **Date:** 2026-07-10

## Context

Player-hosting (and any per-container game selection) needs a fresh xemu instance
to run a chosen game with **no per-container dashboard/game config and no manual
launch**. The prior master image (`_default.qcow2`) installs Halo on the HDD and
exposes it through the UnleashX "Play Halo" menu entry — a manual, single-game
path. The design intent ("attach the selected XISO as the disc… a disc dodges the
media-type check") is to attach a per-instance game ISO as the DVD and have the
Xbox boot it directly, so the overlay stays lean (no game on the HDD) and the
game is chosen per instance.

Two mechanisms were on the table:

1. **Attach the ISO as the DVD** (`DVDPath` knob → xemu `dvd_path`), relying on the
   Xbox to auto-run a game disc present at boot.
2. **Bake a one-time auto-launch config into the master root** — set UnleashX's
   `<DVD AutoLaunch="Yes">` in `E:\Dashboard\config.xml` (or a Cerbios
   `cerbios.ini` disc-auto-boot), so the dashboard auto-runs the disc instead of
   sitting idle.

The open question was whether (1) alone suffices or whether (2) is required.

## Decision

**Ship (1); do NOT apply (2).** Attaching a per-instance game ISO as the DVD is
the complete, sufficient mechanism.

- The podman provisioner accepts a per-instance ISO (`CreateOptions.GameISO`,
  resolved against `Config.ISODir` or the global `Config.DVDPath`) and bind-mounts
  it **read-only** at `/game.iso`. The overlay never carries the game.
- The init script `containers/xemu/init/02-patch-toml.sh` patches
  `dvd_path = '/game.iso'` under `[sys.files]` when that file is present, so xemu
  **attaches the disc** at boot. (Previously the ISO was mounted but xemu never
  attached it — the missing wire.)
- The master image is left **unmodified**. Live A/B on the unmodified
  `_default.qcow2` (2026-07-10, verified by reading the running XBE certificate
  title over QMP `memsave`):

  | Disc attached | Running XBE (title) |
  | ------------- | ------------------- |
  | none          | `UnleashX` (dashboard) |
  | `Halo CE.iso` | `Halo`               |
  | `Halo 2.iso`  | `Halo 2`             |

  A game disc present at cold boot is auto-launched by the master's Cerbios/Xbox
  boot path **regardless of UnleashX's `<DVD AutoLaunch>` toggle** (which was
  `"No"` throughout). `Halo 2.iso` booting Halo 2 — with only Halo CE installed on
  the HDD — proves the **disc** is the boot source, not the HDD install.

The auto-launch lever from mechanism (2) is **built and unit-/live-tested but not
applied and not wired into `Create`**: `cmd/master-autolaunch` +
`Manager.SetMasterDVDAutoLaunch` flip UnleashX's `<DVD AutoLaunch>` to `"Yes"` in
the root via the rootless `qemu-storage-daemon` FUSE + pyfatx path (the same
mechanism `console_name.go` uses; generic helper `containers/xemu/tools/fatx_file.py`).
It exists for a future image that *does* idle on the dashboard with a disc, or to
auto-run a disc inserted while the dashboard is up.

## Consequences

- **Positive:** the clean "attach ISO → game" path works on the stock master with
  zero master edits — no overlay-invalidating root modification, no per-container
  config. Per-instance game selection is a single bind-mount + a TOML line. The
  overlay stays lean (game read from the read-only ISO, never copied).
- **Positive:** verification is definitive — the running XBE certificate title
  (read over QMP `memsave`, rootless) names the exact program running, so
  "booted into the game" is not inferred from an ambiguous game-global read.
- **Negative / trade-off:** we keep an unused lever (`cmd/master-autolaunch` +
  `master_autolaunch.go`). Justified by the project's history of container-image
  drift and because it's a manual, opt-in tool with zero cost when unused.
- **Operational:** editing the shared root (if the lever is ever used) invalidates
  every overlay backed by it (qcow2 backing-chain rule) — rebuild instances after.
  Reading/writing the root's FATX config uses the FUSE + pyfatx path; pyfatx must
  be importable by the configured python (`CONTAINERS_PYTHON_CMD`).
- **Caveat:** the container xemu's QMP strips `screendump`, so boot confirmation
  uses `memsave`/`pmemsave` memory reads (both present), not framebuffer grabs.
