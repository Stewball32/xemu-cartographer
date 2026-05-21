# Milestone 3 — Container lifecycle (Podman)

This is load-bearing — the product has no real UX without it.

**Status:** Ported early. `internal/podman/`, `internal/discovery/`, the six `/api/admin/containers/*` HTTP handlers, env-driven config, and the `CONTAINERS_PODMAN_CMD` rooted-podman escalation are all in. End-to-end create + start + stop + delete + QMP-socket discovery has been smoke-tested against real containers. Two items remain (see follow-ups below): the discovery → scraper auto-start callback (depends on M1+M2) and the `jlesage/firefox` kiosk container's X11 init issue.

- Copy `containers/xemu/init/{01-setup-toml.sh,02-patch-toml.sh,03-setup-hdd.sh,.env}` verbatim into the new repo's `containers/xemu/init/`.
- Port `internal/podman/{podman.go,ports.go,state.go,ports_test.go}` as-is (clean, no known bugs).
- Port `internal/discovery/` socket-directory watcher; wire it to the scraper registry so new `.sock` files in the shared QMP dir auto-start a scraper.
- Port the 6 `/api/containers/*` HTTP handlers from legacy `cmd/cartographer/main.go` into a new `internal/pocketbase/routes/containers.go`. Adapt to PocketBase's `ServeMux` and add the template's auth middleware (legacy assumed localhost-only).
- Extend `xemu-cartographer.toml.example` or fold container config into the root `.env` / a new `config.toml`; decide during porting.
- **Smoke test:** POST `/api/containers` creates an instance → POST `/start` boots xemu + browser containers → scraper auto-connects → live data flows → POST `/stop` + DELETE tears down cleanly.

## M3 follow-ups (deferred)

- ~~**Browser kiosk Firefox crashes inside `jlesage/firefox` container.**~~ Resolved — root cause was the host's OCI runtime, not the image. With `runc` 1.4.x as podman's runtime, jlesage's Xvnc rejects every X client with `Authorization required, but no authorization protocol specified` and Firefox + xcompmgr never connect; with `crun` the same image bits work cleanly. [.env.example](../.env.example) now defaults `CONTAINERS_PODMAN_CMD=sudo -n podman --runtime=crun` and the [CLAUDE.md "Containers" prereq](../CLAUDE.md) requires `sudo pacman -S crun`.
- ~~**Discovery → scraper auto-start wiring.**~~ Done — `cmd/server/main.go` wires `discovery.NewWatcher` `onAdd`/`onRemove` directly to `scrMgr.Start`/`Stop`, swallowing already-running errors so manual + watcher paths coexist.
