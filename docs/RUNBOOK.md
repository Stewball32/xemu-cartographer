# Runbook

Step-by-step procedures for operating a deployed instance. Each section is a self-contained recipe — copy/pasteable commands, expected output, and recovery steps if something goes wrong.

> Keep procedures here when you find yourself looking up the same steps twice. If a section grows beyond ~one screen, split it into `runbooks/<name>.md`.

## Deploy a new release

The release workflow is a single sequence — `git tag` is the trigger, everything else propagates from it. See [ADR-0001](decisions/0001-version-from-git-tag.md) for the mechanism.

1. **Move `CHANGELOG.md` `[Unreleased]` entries** into a new section `[X.Y.Z] - YYYY-MM-DD`. Pick `X.Y.Z` per SemVer:
   - `feat` → minor bump
   - `fix` → patch bump
   - `feat!` / `BREAKING CHANGE:` footer → major bump
2. **Commit:** `git commit -m "chore(release): vX.Y.Z"`.
3. **Tag:** `git tag vX.Y.Z && git push origin main vX.Y.Z`.
4. **Build:**
   - Binary: `task build` → `./bin/server` self-reports the new version.
   - Container: `task container:build` → image tagged `yourproject:vX.Y.Z` and `yourproject:latest`.
5. **Verify locally:**

   ```sh
   ./bin/server serve &
   curl http://localhost:8090/api/version
   # {"version":"vX.Y.Z","commit":"abc1234","date":"..."}
   ```

6. **Deploy** per your environment (push the image to a registry, restart the service, etc.).

> If you tag without changing source, `task build` may skip the rebuild because Task's source-cache doesn't track the git tag. Workaround: `task clean && task build`, or touch any `.go` file.

## Back up `pb_data`

_TODO — fill in. Should cover: where the volume lives, how to snapshot it, where backups are stored, retention._

## Restore `pb_data` from backup

_TODO — fill in. Should cover: stopping the service, replacing the volume, verifying ownership (UID 1000), restarting, verifying admin login._

## Rotate secrets

_TODO — fill in. Should cover: which env vars exist, where they're set in production, how to roll them without downtime (or with planned downtime)._

## Recover from "PocketBase admin locked out"

_TODO — fill in. PocketBase has a CLI for resetting admin password; document the exact command and any caveats around running it against a live container._

## Reset the dev environment

```sh
task clean              # wipes bin/, tmp/, pb_public/
cd sveltekit && pnpm install
task dev                # fresh seed each run (Air uses -tags dev)
```

Dev DB is ephemeral — `tmp/pb_data/` is wiped by Air on exit. See CLAUDE.md for the full story.
