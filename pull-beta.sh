#!/usr/bin/env bash
#
# pull-beta.sh — refresh the BETA run dir with the latest build of the `beta`
# branch, WITHOUT starting anything.
#
# This is the companion to deploy-beta.sh for the "Stewart runs the process"
# workflow: deploy-beta.sh is the full stop → install → start → health-check
# cycle, while pull-beta.sh only does the BUILD + INSTALL half and then hands
# off. Use it when the tier is stopped (or you want to restart it yourself).
#
# WHY THIS EXISTS / HOW THE TIER ACTUALLY GETS ITS CODE
#   ~/xcarto-beta is a standalone SNAPSHOT dir — not a git checkout, and
#   run-beta.sh does NOT build anything. Its last line is:
#       exec ./server serve --http=127.0.0.1:<port>
#   so the tier runs a PRE-BUILT binary that must be copied in from this repo.
#   Nothing reaches beta until this script (or deploy-beta.sh) copies it there.
#
# WHAT IT WRITES (everything else in the run dir is left alone)
#   server            the production binary, built from this working tree
#   pb_public/        the static frontend, built for the tier's port
#   tools/game-maps/  the vendored Python BSP renderer — a RUNTIME dep of map
#                     thumbnails (MAPS_THUMBS_SCRIPT defaults to
#                     ./tools/game-maps/extract_bsp.py, relative to the run dir)
#   run-beta.sh       only if missing, so a wiped dir is reproducible
#   BUILD-INFO        what was installed, from which commit, when
#
# WHAT IT NEVER TOUCHES (externally provided / tier state)
#   .env  .overlay_secret  containers/  pb_data/  inbox/
#
# Usage:
#   ./pull-beta.sh            build the local `beta` branch → install
#   ./pull-beta.sh --fetch    fast-forward from origin/beta first, then build
#   ./pull-beta.sh --help
#
# Overrides:
#   ALLOW_ANY_BRANCH=1   build from a branch other than BETA_BRANCH
#   BETA_DIR=/path       target a different run dir
set -euo pipefail
cd "$(dirname "$0")"
# shellcheck source=scripts/deploy-lib.sh
. scripts/deploy-lib.sh

TIER="beta (test)"
BETA_BRANCH="${BETA_BRANCH:-beta}"
BETA_DIR="${BETA_DIR:-$HOME/xcarto-beta}"
RUNNER="run-beta.sh"

FETCH=0
case "${1:-}" in
  --fetch) FETCH=1 ;;
  -h|--help)
    sed -n '2,40p' "$0" | sed 's/^# \{0,1\}//'
    exit 0
    ;;
  "") ;;
  *) die "unknown argument: $1 (try --help)" ;;
esac

require_cmd git go pnpm
require_dir "$BETA_DIR" "the beta run dir is missing — see docs/DEPLOYMENTS.md"
require_file "$BETA_DIR/.env" "copy .env.example to $BETA_DIR/.env and fill it in"

# Port comes from the tier's own .env (single source), falling back to PORTS.md.
PORT="$(grep -E '^PUBLIC_PB_PORT=' "$BETA_DIR/.env" 2>/dev/null | cut -d= -f2- | tr -d '"' || true)"
PORT="${PORT:-18099}"

say "── ${TIER}: pull latest ──────────────────────────────────"
info "source repo : $PWD"
info "run dir     : ${BETA_DIR}"
info "host port   : 127.0.0.1:${PORT}"
say

# --- source state -------------------------------------------------------------
require_branch "$BETA_BRANCH"

if [ "$FETCH" = "1" ]; then
  say "── fetching origin/${BETA_BRANCH} ────────────────────────"
  git fetch origin "$BETA_BRANCH" || die "git fetch failed"
  # Fast-forward only: never rewrite or merge local work implicitly.
  if git merge-base --is-ancestor HEAD "origin/$BETA_BRANCH" 2>/dev/null; then
    git merge --ff-only "origin/$BETA_BRANCH" || die "fast-forward failed"
    ok "fast-forwarded to origin/${BETA_BRANCH}"
  elif git merge-base --is-ancestor "origin/$BETA_BRANCH" HEAD 2>/dev/null; then
    info "local ${BETA_BRANCH} is AHEAD of origin (unpushed commits) — building local"
  else
    warn "local ${BETA_BRANCH} has DIVERGED from origin — building local, reconcile manually"
  fi
fi

COMMIT="$(git rev-parse --short HEAD)"
SUBJECT="$(git log -1 --pretty=%s)"
ok "commit: ${COMMIT} — ${SUBJECT}"

# Dirt that actually CHANGES WHAT SHIPS: Go sources, module files, migrations,
# and frontend sources. Scoped deliberately — this repo permanently carries
# uncommitted scratch (rig scripts, notes) that never reaches the artifacts, and
# a warning that always fires is a warning nobody reads.
DIRTY="$(git status --porcelain --untracked-files=no -- \
  '*.go' go.mod go.sum migrations/ sveltekit/src/ sveltekit/package.json || true)"
if [ -n "$DIRTY" ]; then
  warn "building with UNCOMMITTED changes to build inputs:"
  printf '%s\n' "$DIRTY" >&2
fi

# --- refuse to install under a live tier --------------------------------------
# Overwriting the binary of a RUNNING process is a footgun: the kernel keeps the
# old inode mapped, so the tier keeps serving stale code and the next restart is
# what actually changes behaviour — exactly the "I restarted and saw no change"
# confusion this script exists to prevent.
RUNNING_PID="$(tier_pid "$PORT")"
if [ -n "$RUNNING_PID" ]; then
  die "${TIER} is RUNNING (pid ${RUNNING_PID} on :${PORT}). Stop it first, then re-run:
    kill ${RUNNING_PID}
  (or use ./deploy-beta.sh, which stops, installs, restarts and health-checks.)"
fi
ok "tier is stopped — safe to install"

# --- build --------------------------------------------------------------------
build_artifacts "$PORT"

# --- install ------------------------------------------------------------------
say "── installing → ${BETA_DIR} ──────────────────────────────"
[ -f "$STAGED_BIN" ] || die "no staged binary — build failed?"
cp "$STAGED_BIN" "$BETA_DIR/server"
rm -f "$STAGED_BIN"
ok "server binary installed"

rm -rf "$BETA_DIR/pb_public"
cp -r pb_public "$BETA_DIR/pb_public"
ok "pb_public installed ($(find "$BETA_DIR/pb_public" -type f | wc -l) files)"

# Map thumbnails shell out to this vendored renderer at runtime — it lives in
# scripts/ in the repo but must land in tools/ in the run dir (the default
# MAPS_THUMBS_SCRIPT path). __pycache__ is build residue; don't ship it.
rm -rf "$BETA_DIR/tools/game-maps"
mkdir -p "$BETA_DIR/tools"
cp -r scripts/game-maps "$BETA_DIR/tools/game-maps"
rm -rf "$BETA_DIR/tools/game-maps/__pycache__"
ok "tools/game-maps installed (map-thumbnail renderer)"

# run-beta.sh isn't tracked in the repo (it's runtime-dir config), so regenerate
# it if the dir was wiped — keeps a bare run dir reproducible from this script.
if [ ! -f "$BETA_DIR/$RUNNER" ]; then
  cat > "$BETA_DIR/$RUNNER" <<RUNNER_EOF
#!/usr/bin/env bash
# Beta / preview xemu-cartographer. Regenerated by pull-beta.sh.
#
# PREVIEW ONLY. Completely separate from prod:
#   prod : /var/lib/xemu-cartographer   :8099   lan.norcal.pro
#   beta : this dir (~/xcarto-beta)     :${PORT}  beta.norcal.pro
#
# Start:  ./run-beta.sh              (foreground)
#         nohup ./run-beta.sh >beta.log 2>&1 &   (background)
# Stop:   kill the PID listening on :${PORT}
set -euo pipefail
cd "\$(dirname "\$0")"

ENV_FILE="\$PWD/.env"
if [ ! -f "\$ENV_FILE" ]; then
  echo "run-beta.sh: \$ENV_FILE not found — copy .env.example to .env and fill it in." >&2
  exit 1
fi
set -a
. "\$ENV_FILE"
set +a

# Bind loopback only — cloudflared connects to localhost.
exec ./server serve --http=127.0.0.1:${PORT}
RUNNER_EOF
  chmod +x "$BETA_DIR/$RUNNER"
  ok "$RUNNER regenerated (was missing)"
else
  info "$RUNNER present — left as-is"
fi

# --- provenance stamp ---------------------------------------------------------
{
  echo "installed_at : $(date -Is)"
  echo "commit       : ${COMMIT}  ${SUBJECT}"
  echo "branch       : $(git rev-parse --abbrev-ref HEAD)"
  echo "port         : ${PORT}"
} > "$BETA_DIR/BUILD-INFO"

# --- verify what actually landed ----------------------------------------------
# `go build` stamps the VCS revision into the binary, so this proves the INSTALLED
# file was built from this commit — independent of anything the build printed.
say "── verifying installed binary ────────────────────────────"
STAMPED="$(go version -m "$BETA_DIR/server" 2>/dev/null \
  | awk '$1=="build" && $2 ~ /^vcs\.revision=/ { sub(/^vcs\.revision=/, "", $2); print $2 }')"
HEAD_FULL="$(git rev-parse HEAD)"
if [ -z "$STAMPED" ]; then
  warn "no vcs.revision stamp found — cannot prove provenance (unexpected)"
elif [ "$STAMPED" = "$HEAD_FULL" ]; then
  ok "binary built from ${COMMIT} (vcs.revision matches HEAD)"
else
  die "installed binary reports vcs.revision ${STAMPED}, expected ${HEAD_FULL}"
fi

# Go stamps vcs.modified=true whenever the repo has ANY uncommitted tracked
# file — including scratch that never feeds the build — so the stamp alone
# can't answer "is the running code exactly this commit?". Pair it with the
# build-input check above, which can.
if [ -n "$DIRTY" ]; then
  warn "binary includes uncommitted BUILD-INPUT changes — not reproducible from ${COMMIT} alone"
  echo "build_inputs : DIRTY (see warning above)" >> "$BETA_DIR/BUILD-INFO"
elif go version -m "$BETA_DIR/server" 2>/dev/null | grep -q 'vcs.modified=true'; then
  ok "binary is exactly ${COMMIT} (repo carries unrelated uncommitted scratch)"
  echo "build_inputs : clean (unrelated scratch uncommitted)" >> "$BETA_DIR/BUILD-INFO"
else
  ok "binary is exactly ${COMMIT} — fully clean tree"
  echo "build_inputs : clean" >> "$BETA_DIR/BUILD-INFO"
fi

if [ ! -d "$BETA_DIR/pb_data" ]; then
  warn "no pb_data — first boot will create a FRESH database:"
  info "migrations apply on boot; the superuser is bootstrapped from"
  info "SEED_SUPERUSER_EMAIL / SEED_SUPERUSER_PASSWORD in .env."
  info "Previously ingested ISOs, containers and users are NOT there."
fi

say
ok "beta run dir updated — NOT started (start it yourself):"
info "cd ${BETA_DIR} && sudo ./${RUNNER}"
info "  or background: cd ${BETA_DIR} && sudo nohup ./${RUNNER} >beta.log 2>&1 &"
say
info "then verify:  curl -sf http://127.0.0.1:${PORT}/api/health && cat ${BETA_DIR}/BUILD-INFO"
