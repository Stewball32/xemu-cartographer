# ADR-0001 — Version from git tag, propagated at build time

> **Status:** Accepted
> **Date:** 2026-05-16

## Context

The template ships several artifacts that each have their own version concept:

- The Go binary
- The container image
- `sveltekit/package.json`
- Anything the app surfaces at runtime (a footer, a health endpoint, an "About" panel)

Keeping these in sync by hand always drifts. You tag `v1.2.0` and forget to bump `package.json`. You push a container labeled `latest` with no record of which commit it was built from. The result is that no version field is trustworthy, so you stop trusting any of them.

## Decision

**The git tag is the canonical version.** Nothing else stores a version value; build artifacts read the tag at build time.

Mechanism:

- `Taskfile.yml` computes three values once per build:
  - `VERSION = git describe --tags --always --dirty` (e.g. `v1.2.0`, or `v1.2.0-3-gabc1234-dirty` for an uncommitted build)
  - `COMMIT = git rev-parse --short HEAD`
  - `DATE = date -u +%Y-%m-%dT%H:%M:%SZ`
- The Go binary embeds them via `-ldflags "-X .../internal/version.Version=…"`.
- `internal/version` exposes them as package vars (`Version`, `Commit`, `Date`).
- `/api/version` returns them as JSON.
- The container is tagged `<image>:<version>` and `<image>:latest`. The same three values are passed into the Containerfile via `--build-arg`, so binaries built inside the container self-report correctly.
- The SvelteKit build sets `PUBLIC_APP_VERSION` from the same source. The frontend can read it via `$env/static/public` if it wants to display the version — opting out is the default.
- `sveltekit/package.json` stays at `0.0.0`. npm's `version` field only matters when publishing, which is not a workflow this template supports.

The release workflow becomes a single sequence:

1. Move entries under `CHANGELOG.md` `[Unreleased]` into `[X.Y.Z] - YYYY-MM-DD`.
2. Commit.
3. `git tag vX.Y.Z`.
4. `task build` (or `task container:build`).

## Consequences

**Positive:**

- One source of truth. No way for two artifacts to disagree about what version they are.
- Dirty builds self-identify (`v1.2.0-3-gabc1234-dirty`), which is invaluable when triaging "is this the binary I just built or last week's?"
- The convention works the same in dev, container builds, and CI.

**Negative:**

- The build requires `git` on PATH. Fine for dev and CI; for container builds, the values are passed via `--build-arg` rather than running git inside the container, so the image doesn't need git installed.
- Task's source-based caching tracks `*.go` files but not the git tag. If you `git tag` without changing source, `task build` may skip the rebuild and the embedded version won't update. Workaround: `task clean && task build`, or touch any `.go` file. In practice releases always include a commit (e.g. the CHANGELOG bump), so this rarely bites.

**Neutral:**

- `package.json` version is permanently `0.0.0`. If the frontend later becomes a published npm package, this decision needs to be revisited — supersede with a new ADR.
