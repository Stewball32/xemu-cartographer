# Project meta-docs

This directory is the source of truth for everything *about* the project that isn't code: where we are, where we're going, and why we made the choices we did. The same shape is used in every project derived from this template, so context-switching between projects is cheap.

> **Where to look first:** [`STATUS.md`](STATUS.md) — current state at a glance.

## Layout

```
docs/
├── README.md          ← you are here (the convention itself)
├── STATUS.md          ← current state: Goals / Now / Next / Maybe
├── milestones/
│   ├── README.md      ← summary table of all milestones
│   └── _template.md   ← copy this when starting a new milestone
└── decisions/
    ├── README.md      ← summary table of all ADRs
    └── _template.md   ← copy this when recording a new decision

CHANGELOG.md           ← (repo root) Keep-a-Changelog, SemVer
```

## Naming

| File         | Pattern                    | Example                          |
| ------------ | -------------------------- | -------------------------------- |
| Milestone    | `M??-kebab-name.md`        | `M00-template-cleanup.md`        |
| ADR          | `????-kebab-name.md`       | `0001-use-pnpm.md`               |
| Template     | `_template.md`             | (underscore sorts first)         |

- Milestone numbers are zero-padded to 2 digits. ADR numbers are zero-padded to 4.
- Never skip a number. Abandon a milestone/ADR instead — its file stays, with `Status: Abandoned` or `Status: Deprecated`.

## Dates

Always absolute (`YYYY-MM-DD`). Never relative ("last week", "Thursday"). Relative dates rot the moment you look back.

## Status vocabularies

**Milestones:** `Planned` | `In progress` | `Done` | `Abandoned`

**ADRs:** `Proposed` | `Accepted` | `Superseded by ADR-XXXX` | `Deprecated`

## Immutability rules

- **Milestone `Log` section is append-only.** Never edit past entries. Append a new dated line.
- **ADRs are immutable once `Accepted`.** Don't edit the Context / Decision / Consequences. To change direction, write a *new* ADR that supersedes the old one — and update both Status lines.

## How to add a new milestone

1. Copy `milestones/_template.md` to `milestones/M??-kebab-name.md` (next number).
2. Fill in Goal, Scope, Actions, Verification. Leave the Log with one `created` entry.
3. Add a row to the table in `milestones/README.md`.
4. If this is what you're working on next, update `STATUS.md` → `Now`.

## How to add a new ADR

1. Copy `decisions/_template.md` to `decisions/????-kebab-name.md` (next number).
2. Fill in Context, Decision, Consequences. Status is `Proposed` until you commit to it.
3. When accepted, flip Status to `Accepted` and add a row to `decisions/README.md`.

## How to supersede an ADR

1. Write a new ADR explaining the new direction.
2. In the new ADR's body, link back to the one being replaced.
3. In the **old** ADR, change Status to `Superseded by ADR-XXXX` (and add the date).
4. Update both rows in `decisions/README.md`.

Never delete or edit the old ADR's body — the historical reasoning is the point.

## Update cadence

- **`STATUS.md`** — update whenever "Now" changes. It's the first file anyone (including future-you) reads.
- **`CHANGELOG.md`** — add a line under `[Unreleased]` as you make user-visible changes; cut a versioned section when you ship a release. SemVer.
- **Milestone files** — update Status and append to Log as work progresses.

## `reference/` (local-only)

A gitignored top-level `reference/` directory is the canonical place for legacy code, archived versions, upstream snippets, or anything you want to consult locally without polluting the repo. The directory is in `.gitignore` (`/reference/`), so it never ships in a clone — create it locally when you need it.

Use it for:
- Old versions of the project you're porting from (e.g. an archived TS codebase being rewritten in Go).
- Reference implementations or upstream libraries you're studying.
- Scratch experiments you don't want tracked.

Don't use it for:
- Anything load-bearing for the build. If the project needs it, it goes in the repo.
- Reference *links* — those go in milestone Notes / ADR Context, or a `docs/glossary.md` if you have one.

## `RUNBOOK.md` (operational procedures)

[`RUNBOOK.md`](RUNBOOK.md) collects step-by-step procedures for operating a deployed instance: deploys, backups, restores, secret rotation, common recovery scenarios. One file is fine until it gets long; split into `runbooks/` if it does.

## Conventions

Beyond the doc structure above, the template adopts a small set of cross-cutting conventions. A few rules that show up everywhere are easier to remember than many rules that show up rarely.

### Commit messages — Conventional Commits

`type(scope): subject`

- **Types:** `feat` `fix` `chore` `refactor` `docs` `test` `perf` `style` `ci` `build` `revert`
- **Breaking changes:** add `!` after the type/scope (`feat(api)!: drop /api/me`) or include a `BREAKING CHANGE:` footer
- **Subject:** imperative, lowercase, no trailing period

Pays off because CHANGELOG bumps become mechanical: `feat:` → Added, `fix:` → Fixed, `feat!` or `BREAKING CHANGE` → major.

### TODO / FIXME tied to milestones or ADRs

```
// TODO(M03): wire ws draft engine
// FIXME(ADR-0007): replace stub rate-limit logic
```

Grep finds open work when you start a milestone (`grep -rn 'TODO(M03)' .`), and a milestone's "Done" criteria can include "no `TODO(M03)` left in code." Bare `TODO` is allowed for short-lived in-flight markers — if it should survive past the session, tag it with a milestone or ADR ID.

### Standard Taskfile targets

Every project derived from this template exposes the same target lineup. Names are reserved even when stubbed — muscle memory stays portable.

| Target              | Purpose                                              |
| ------------------- | ---------------------------------------------------- |
| `dev`               | Backend + frontend dev servers                       |
| `build`             | Production build of everything                       |
| `test`              | Run all tests (Go) + frontend type-check             |
| `fmt`               | Format all code                                      |
| `lint`              | Lint all code                                        |
| `clean`             | Remove build artifacts                               |
| `container:build`   | Build container image                                |
| `container:run`     | Run container                                        |
| `install:hooks`     | Install pre-commit hooks (lefthook)                  |
| `typegen`           | Regenerate PocketBase TypeScript types (if relevant) |

### Git tag format

Tags use a leading `v`: `v0.1.0`, `v1.2.3`. Some tooling (Go modules, GitHub releases, `gh` CLI) assumes the prefix; omitting it makes integration awkward.

Versions follow [SemVer](https://semver.org/). The git tag is the source of truth — `task build` embeds the version into the Go binary via ldflags and surfaces it at `/api/version`. See [ADR-0001](decisions/0001-version-from-git-tag.md) for propagation details.

### `.env.example` structure

- Group variables by subsystem with a banner-style header comment, and mark the whole group required or optional in the header itself:
  ```
  # ── Required ──────────────────────────────────────────────────────────────────
  # Minimum needed to boot the dev server.
  PUBLIC_PB_PORT=8090

  # ── Discord Bot (optional) ────────────────────────────────────────────────────
  # Server runs fine without these — set both to enable the bot.
  DISCORD_BOT_TOKEN=
  DISCORD_DEV_GUILD_ID=
  ```
- Alphabetize variables within a group. If a group has stable sub-groupings (e.g. OAuth providers), alphabetize those too.
- When a group mixes required and optional variables, fall back to inline markers (`# required` / `# optional — purpose`).
- Real secrets never land in `.env.example`. Placeholder or example values are fine; document expected format in the header comment if it's non-obvious.

## Optional patterns

These aren't created in fresh clones — add them when you actually need them. The shape is documented here so when the moment comes, you don't reinvent the format.

### `docs/incidents/` — postmortems

When something breaks badly enough that you'd want future-you to know what happened, write an incident report. One markdown per incident, named `YYYY-MM-DD-kebab-name.md`.

Shape:
- Header: Date, Duration, Severity, Authors
- **Summary** — one paragraph, readable in 5 seconds
- **Timeline** — what happened in order (absolute timestamps)
- **Root cause** — technical cause plus contributing factors
- **Resolution** — what actually fixed it
- **Prevention** — checkable follow-up items (these often become milestones or ADRs)
- **Related** — links to relevant ADRs, milestones, commits

Reach for an incident report when (a) the bug is non-obvious and likely to recur in different forms, (b) the fix requires changing how you build, not just patching code, or (c) you'd want a clear record in six months. Run-of-the-mill bugs belong in commit messages.

### `docs/glossary.md` — domain terms

When the project mixes systems (Discord / frontend / backend) that refer to the same concepts with slightly different names, a single glossary file pulls weight. Add it when you find yourself searching for "what did I call X again."
