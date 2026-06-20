# M26 — HDD copy-on-write overlay provisioning

> **Status:** Done
> **Started:** 2026-06-20
> **Completed:** 2026-06-20
> **Depends on:** M03 (container lifecycle)

## Goal

Replace the per-instance **full HDD copy** with a **qcow2 copy-on-write overlay**.
Previously every container got its own ~8.4 GiB `cp` of the root disk (slow to
create, N×8 GiB of disk for N instances). Now one canonical **read-only root**
qcow2 is shared by all instances, and each instance gets a thin overlay that
records only its own deltas. xemu reads the canonical disk from the root through
qcow2's backing chain; the root is never written.

## Scope

- **In:** the big HDD qcow2 image only. Root designation + freeze, per-instance
  overlay create on container create, overlay delete on teardown, init-script
  migration, config + docs.
- **Out:** eeprom / flashrom / bios. There is no per-instance eeprom/flashrom
  provisioning today (xemu uses image-level defaults), so nothing there changes.
- **Out:** auto-migrating the pre-existing full-copy instance disks
  (`nexy.qcow2`, `stew.qcow2`, `test*.qcow2`). They keep working as standalone
  images; an operator can delete them to reclaim space and re-create the
  instances as overlays.

## Design decisions

- **Root = `_default.qcow2`** in `containers/xemu/shared/hdds/` (configurable via
  `CONTAINERS_ROOT_HDD`, kept in sync with `DEFAULT_HDD_NAME` in
  `containers/xemu/init/.env`). It is the canonical Halo-installed disk.
- **Provisioning is host-side, in Go** (`internal/podman`, `overlay.go`), not in
  the container init script. The host always has `qemu-img`; the linuxserver/xemu
  image is not guaranteed to. `Manager.Create` calls `provisionOverlay`; the init
  script (`03-setup-hdd.sh`) now only injects the overlay's `hdd_path` into the
  toml (with an in-container `qemu-img` fallback for standalone runs — never a
  full copy).
- **Relative backing path.** The overlay stores its backing reference as the bare
  basename (`_default.qcow2`), not an absolute path. Root and overlays share the
  `hdds` dir, which is bind-mounted at a *different* absolute path inside the
  container (`/shared/hdds`) than on the host (`./containers/xemu/shared/hdds`).
  A relative backing resolves correctly in both, and survives the instances dir
  being relocated, as long as the root stays alongside its overlays.
- **Root is frozen read-only (0444)** by `freezeRoot` on first overlay creation.
  Modifying a backing file corrupts every overlay above it. qemu already opens
  backing files read-only; the chmod makes accidental host-side edits fail loudly
  and ensures every overlay reader can read the shared backing. `CleanupOrphans`
  already skips `_`-prefixed baselines, so the root is never auto-deleted.

  **⚠️ If you update the root, you MUST rebuild every overlay.** Delete all
  instance overlays, replace (or `chmod +w` then overwrite) the root, then
  re-create the instances. There is no in-place root edit that preserves
  overlays.

## Actions

- [x] Add `CONTAINERS_ROOT_HDD` + `CONTAINERS_QEMU_IMG_CMD` config (defaults
      `_default.qcow2` / `qemu-img`).
- [x] `internal/podman/overlay.go`: `provisionOverlay` (relative backing) +
      `freezeRoot` (0444) + `removeOverlayFile`.
- [x] Wire `provisionOverlay` into `Manager.Create` (before container create) and
      `removeOverlayFile` into `Manager.Remove`. (`DeleteFiles` / `CleanupOrphans`
      already glob `hdds/<name>.*`, so they delete overlays too.)
- [x] Migrate `03-setup-hdd.sh` from `cp` to overlay-aware path injection +
      in-container `qemu-img` fallback (no full-copy fallback).
- [x] Unit tests (`overlay_test.go`): relative-backing store, root freeze,
      reuse-idempotency, N overlays share one root, overlay delete keeps root,
      missing-root error.
- [x] Docs: CLAUDE.md (baseline + podman row + qemu-img prereq), README, CHANGELOG.

## Verification

Proven on the host (`qemu-img` 11.0.1, `xemu` 0.8.136 / QEMU 10.2 core) before
and after the implementation:

- **Backing-chain mechanics** (`qemu-io`): an overlay reads the root's data
  through the chain; a write to the overlay isolates to that overlay (other
  overlays don't see it); the root stays **byte-identical** (sha256 unchanged)
  and **rejects writes** at 0444.
- **xemu honors the chain (real boot):** booted `xemu` against an overlay of the
  real `_default.qcow2` — QMP `query-status` returned `running`, the overlay grew
  ~642 KiB of real disk deltas, and the root's size+mtime were unchanged. The
  stored backing string was the relative `_default.qcow2`, resolved to the root.
- **N concurrent overlays over one root:** 6 overlays written simultaneously,
  each isolated, root unchanged after all 6.
- **Go path:** `go test ./internal/podman/` — overlay create stores relative
  backing, freezes root 0444, is idempotent, deletes overlay while keeping root,
  errors on missing root. `go build ./...` + `go vet ./...` green.

## Log

_Append-only. Never edit past entries; add a new dated line._

- 2026-06-20: created + implemented. Proved qcow2 CoW backing chain + a real xemu
  overlay boot honor the root read-only; moved provisioning host-side into
  `internal/podman/overlay.go`; migrated `03-setup-hdd.sh` off `cp`; froze root
  at 0444; relative backing path chosen for host/container path-parity.
