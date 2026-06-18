# Overnight progress log

> Autonomous session started 2026-06-16. Stewart asked for maximum roadmap
> progress overnight: complete M09's reasonable scope, then continue M10, M11,
> onward — each milestone implemented, tested, docs updated, committed locally
> on a stacked branch. **Nothing pushed / merged / PR'd** — all local for review.

This file is the running decision + status log. Read the **Branch stack** and
**Needs Stewart's review** sections first.

## Branch stack (bottom → top, nothing merged to main)

`main` (M08 merged) → `wip/milestone-9` (M09) → `wip/milestone-10` (M10) → …

Each milestone branches off the previous one's tip. To review in order, walk the
stack bottom-up. Current tip is recorded in the **Status** table below.

## Status

| Milestone | Branch | State | Tests | Notes |
| --------- | ------ | ----- | ----- | ----- |
| M09 — Match-aware kiosk | `wip/milestone-9` | code-complete | green | Live 4-container smoke test can't run here (no podman) |

## Environment notes (for reproducing my green checks)

- **Use scoped Go commands:** `go build/vet/test ./cmd/... ./internal/...`. A
  bare `./...` walks `containers/browser/config-*` (root-owned leftovers from
  earlier container smoke runs) and dies on `permission denied`. CI uses a clean
  checkout so `./...` is fine there; locally the scoped form is equivalent (all
  Go code lives under `cmd/` + `internal/`).
- Frontend checks run from `sveltekit/`: `pnpm check && pnpm lint && pnpm test && pnpm build`.
- `seed.local.json` is an untracked local file (not mine) — left in place; I use
  explicit `git add <paths>` so it never lands in a commit.
- Stashed on entry: `git stash` "overnight: feat/json-seeder uncommitted seed
  work (02-patch-toml.sh, seed.example.json)" — the working tree was on
  `feat/json-seeder` with 2 uncommitted files when I started; stashed them so I
  could switch to `wip/milestone-9`. Recover with `git stash pop` on
  `feat/json-seeder`.
- Side branches I did **not** touch: `feat/json-seeder`, `chore/align-dev-seed-creds`
  (Stewart's 1-commit dev-seed chore off `main`).

## Decisions made (autonomous — flag anything you'd have called differently)

- M09 stays as committed (`57c566e`), including the WS `host:<name>` room
  narrowing + the kiosk/VNC proxy narrowing. The branch-switch "file modified"
  notes were just the harness observing the checkout, not a revert.

## Needs Stewart's review (does not block overnight work)

- **M09 security boundary:** opening the kiosk HTTP proxy + VNC relay to
  non-admin roster members (`authorizeKioskAccess`). Same predicate also opens
  `host:<name>` WS rooms to roster members. Unit-tested + fails closed, but the
  live stream to a non-admin is unverified (needs the podman smoke test).

## Per-milestone log

### M09 — Match-aware kiosk view (first increment) — `wip/milestone-9`
Committed before this session (`57c566e`). Code-complete; only the live
4-container smoke test remains (podman-gated). No further code changes needed.
