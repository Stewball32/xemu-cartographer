# Deployments

How cartographer is deployed, under the host's **/srv Hosting Standard**
(`/srv/shared/HOSTING-PLAN.md`; canonical registries in `/srv/registry/` —
[`SITES.md`](/srv/registry/SITES.md) + [`PORTS.md`](/srv/registry/PORTS.md)).
Three tiers. Deployed tiers hold **artifacts + state only** (binary, `pb_data/`,
`.env`) — never a git checkout; builds happen in the repo, installs land under
`/srv`.

> **Cartographer runs NATIVELY, not in Docker.** It provisions xemu containers
> and needs host podman, `/dev/kvm`, `/dev/dri` and raw sockets, so each tier is
> a plain binary with its own run dir, `.env`, port and `pb_data`.
>
> Since step 8 a tier is **two** native processes: the league (`bin/server`) and
> the **xc-scraper daemon** (`bin/xc-scraper`, DESIGN-STEP8 §11 / D-14). The
> daemon is not containerised either — it needs the QMP socket directory,
> `open("/proc/<pid>/mem")` of every xemu process and same-host PIDs; in a
> container that would mean `pid=host` + privileged + host networking, which buys
> nothing over a plain binary. The `Containerfile` image is therefore
> **league-only**; a container deployment still needs the daemon on the host.

## Tiers

| Tier | Purpose | Runs from | How | Port | Hostname | Bot |
| --- | --- | --- | --- | --- | --- | --- |
| **dev** | live-reload coding | this working tree (Air + Vite HMR) | `./run-dev.sh` | `19099` vite / `19090` backend | `lab.norcal.pro` (only while up) | **off** (forced) |
| **pre** (test) | the gate before prod — merged branches soak here | `/srv/http/xemu-cartographer/pre/` | `/srv/registry/srv-pre.sh xemu-cartographer [ref]` → systemd **user** unit `site-xemu-cartographer-pre` | `18099` | `beta.norcal.pro` | cart-beta **test** app (per tier `.env`) |
| **prod** | the live site | `/srv/http/xemu-cartographer/prod/` | systemd unit `site-xemu-cartographer-prod` (migrated 2026-08-27; still root + `0.0.0.0` — de-root/loopback is a pending follow-up) | `8099` | `lan.norcal.pro` | cart **prod** app |

- Ports are claimed in the **canonical host registry**
  `/srv/registry/PORTS.md` (cartographer's spread — `8099`, `18099`,
  `19090/19099`, rig reservations `3300-3399` tcp + `9970-9989` udp — is
  grandfathered; don't renumber). Audit with `sudo /srv/registry/check-ports.sh`.
- Tiers bind **loopback only** (prod's `0.0.0.0` is the known exception being
  fixed); cloudflared is the public front door, the LAN Caddy serves
  `lan.local`.
- Data is separate per tier: each run dir has its own `pb_data/`. Dev's is
  ephemeral (`tmp/pb_data`, wiped on exit).
- **Discord: one gateway per bot token** — each tier that runs the bot needs its
  own Discord application; dev never runs it.
- Naming hazard: prod `lan.norcal.pro` and dev `lab.norcal.pro` are one letter
  apart in the same ingress list — double-check which you're editing.

### The xc-scraper daemon per tier

| Tier | Daemon port | Unit | Webhook target (league) | State dir |
| --- | --- | --- | --- | --- |
| **dev** | `8992` | none — `task dev` runs it under Air (`dev:scraper`) | `http://127.0.0.1:$PUBLIC_PB_PORT/api/xc/finished_game` | `../xc-scraper/tmp/xc-scraper-state` |
| **pre** | `8991` | `xc-scraper-pre` (system unit, root) | `http://127.0.0.1:18099/api/xc/finished_game` | `<tier>/xc-scraper-state/` |
| **prod** | `8990` | `xc-scraper-prod` (system unit, root) | `http://127.0.0.1:8099/api/xc/finished_game` | `<tier>/xc-scraper-state/` |

- The unit template and install steps live in the sibling repo:
  [`../xc-scraper/deploy/xc-scraper.service`](../../xc-scraper/deploy/xc-scraper.service)
  + [`../xc-scraper/deploy/README.md`](../../xc-scraper/deploy/README.md)
  (`sed` the `<tier>` / `899X` / `<web port>` placeholders). It runs as root because
  the podman stack runs xemu as root (README "Memory access").
- `task build` produces **both** `bin/server` and `bin/xc-scraper`. The install
  script (`srv-pre.sh`) does **not** copy `bin/xc-scraper` yet — extending it is an
  owner gate; until then copy the binary into the tier dir by hand and restart the
  `xc-scraper-<tier>` unit (see "Deploying" below). `bin/xc-scraper --version`
  prints the same `git describe` string as the league's `BUILD-INFO`.
- The daemon's ports `8990-8992` are a reversible default (D-14) and are **not yet
  claimed** in `/srv/registry/PORTS.md` — owner action, together with the unit
  install and the `srv-pre.sh` sibling checkout (both outside this repo).
- The daemon and the league share the tier's `.env` (`EnvironmentFile`): the daemon
  reads `XC_SCRAPER_TOKEN` / `XC_SCRAPER_CONTROL_TOKEN` / `XC_SCRAPER_WEBHOOK_TOKEN`
  (+ any `XC_SCRAPER_<FLAG>` twin), the league reads the same three plus
  `XC_SCRAPER_URL=http://127.0.0.1:<daemon port>` — the switch that puts it in
  wire mode (R1). No systemd ordering between the two units: the league reconnects
  forever and the daemon spools finished games until the webhook answers 2xx.
- **Cut-over / rollback order** (the one hard rule — two attachers on one QMP
  directory persist every game twice): cut over by booting the league with
  `XC_SCRAPER_URL` set, then starting the daemon; roll back by stopping the daemon
  **first**, then restarting the league without `XC_SCRAPER_URL`. Step list in
  [RUNBOOK.md](RUNBOOK.md) ("Switch the scraper feed to the xc-scraper daemon").

## Promotion path

```
dev (working tree)  ──►  pre (/srv, :18099)  ──►  prod (/srv, :8099)
   run-dev.sh            srv-pre.sh               site-xemu-cartographer-prod
```

Nothing should reach prod without soaking on pre. For anything touching the
schema, pre applies the migration first — see [MIGRATIONS.md](MIGRATIONS.md).

## Deploying pre

```sh
/srv/registry/srv-pre.sh xemu-cartographer beta     # build the beta branch tip
/srv/registry/srv-pre.sh xemu-cartographer <ref>    # any committish
/srv/registry/srv-pre.sh xemu-cartographer status|logs|restart|stop|info
```

What it does (see the script header for the full contract): builds the ref in a
**temporary detached worktree** (only committed code ships — the repo checkout
is never touched; **since the step 7 restructure `go.mod` resolves
`github.com/xemu-cartographer/xc-scraper` via `replace => ../xc-scraper`, so the
worktree's parent directory must also hold an `xc-scraper` checkout at the
matching ref. `task container:build` handles this by passing the sibling as a
named build context (`podman build --build-context xc-scraper=../xc-scraper`,
see `Containerfile`; `.containerignore` keeps `sveltekit/node_modules`, `pb_data`,
`.env*` etc. out of the context — without it the frontend stage's `pnpm build`
aborts on the host's `node_modules`); `srv-pre.sh` lives in `/srv/registry`, outside this repo,
and does NOT check the sibling out yet — until it does, a pre deploy from this
branch fails at `go build` with "replacement directory ../xc-scraper does not
exist". The sibling is the **private** org repository
`github.com/xemu-cartographer/xc-scraper` (since 2026-09-15), so whatever the
script uses to clone it — a deploy key or a fine-grained PAT with Contents:
read-only on that one repo — must be provisioned on the host first; the
flagship's CI uses the `XC_SCRAPER_CHECKOUT_TOKEN` Actions secret for the same
purpose**), installs `bin/server` + `pb_public/` + `tools/game-maps/`
into the tier, regenerates `run.sh`, writes `BUILD-INFO` with provenance
verification, then (re)starts the `site-xemu-cartographer-pre` **user** unit
(linger is on — survives reboots, no sudo) and polls `/api/health`. It never
touches `.env` or `pb_data/`; a missing `.env` is seeded once from the repo's
`.env.pre` / `*.example` and must be reviewed. **It does not yet install
`bin/xc-scraper` or restart the `xc-scraper-pre` unit** (step 8): until the
owner extends it, copy `bin/xc-scraper` into the tier and
`sudo systemctl restart xc-scraper-pre` by hand after a pre deploy — the daemon
spools finished games across its own restart, so the order does not matter.

**Pending migrations apply on boot**, so a healthy check also proves the
migrations applied.

## Rollback

```sh
/srv/registry/srv-pre.sh xemu-cartographer <last-good-ref>
```

⚠️ Code rolls back; **migrations do not**. An applied migration stays applied —
that's why they're proven on pre first. To undo a schema change, write a new
forward migration.

## Backups

`/srv/backups/` + a nightly snapshot timer is a planned phase of the hosting
standard (not live yet). Until then, copy a tier's `pb_data` yourself before
risky migrations: `cp -r <tier>/pb_data <tier>/pb_data.bak-$(date +%Y%m%d)` —
and prune old copies periodically; they are full copies.

## History

The pre-/srv deployment generations are retired: `deploy-beta.sh` (in-repo
full-cycle wrapper), then `~/xcarto-beta` + `pull-beta.sh` (home-dir run dir,
manual start), then briefly a hand-rolled `/srv/.../pre` on an unregistered
port. All superseded by `srv-pre.sh` + systemd units per the hosting standard.
