#!/usr/bin/env bash
# Shared deploy helpers — sourced by deploy-beta.sh so every cartographer tier
# behaves identically: guard → build → deploy → health-check.
#
# Same INTERFACE as the stew-site-template standard (deploy-lib.sh), but
# cartographer runs NATIVELY, not in Docker: it needs host podman, /dev/kvm,
# /dev/dri and raw sockets for the xemu containers it provisions, so the tiers
# are plain processes with their own run dir + .env instead of compose stacks.
# The guards, output, and health-check contract are identical.
#
# Not executable on its own.

set -euo pipefail

# --- output ------------------------------------------------------------------
_c_red=$'\033[31m'; _c_grn=$'\033[32m'; _c_ylw=$'\033[33m'; _c_off=$'\033[0m'
say()  { printf '%s\n' "$*"; }
info() { printf '  %s\n' "$*"; }
ok()   { printf '%s✓%s %s\n' "$_c_grn" "$_c_off" "$*"; }
warn() { printf '%s!%s %s\n' "$_c_ylw" "$_c_off" "$*" >&2; }
die()  { printf '%s✗%s %s\n' "$_c_red" "$_c_off" "$*" >&2; exit 1; }

# --- guards ------------------------------------------------------------------

# require_cmd <cmd>...
require_cmd() {
  local c
  for c in "$@"; do
    command -v "$c" >/dev/null 2>&1 || die "required command not found: $c"
  done
}

# require_file <path> <hint>
require_file() {
  [ -f "$1" ] || die "missing $1 — $2"
}

# require_dir <path> <hint>
require_dir() {
  [ -d "$1" ] || die "missing $1 — $2"
}

# require_branch <expected>  (skip with ALLOW_ANY_BRANCH=1)
require_branch() {
  local want="$1" have
  have="$(git rev-parse --abbrev-ref HEAD)"
  if [ "$have" != "$want" ]; then
    [ "${ALLOW_ANY_BRANCH:-0}" = "1" ] \
      && { warn "on '$have', expected '$want' (ALLOW_ANY_BRANCH=1)"; return 0; }
    die "on branch '$have', expected '$want'. Checkout $want, or re-run with ALLOW_ANY_BRANCH=1."
  fi
  ok "branch: $have"
}

# require_clean_tree  (skip with ALLOW_DIRTY=1)
# Deploys build from the working tree, so a dirty tree means shipping something
# that isn't committed — you couldn't reproduce or roll back to it.
require_clean_tree() {
  if [ -n "$(git status --porcelain)" ]; then
    [ "${ALLOW_DIRTY:-0}" = "1" ] \
      && { warn "working tree is dirty (ALLOW_DIRTY=1)"; return 0; }
    git status --short >&2
    die "working tree is dirty. Commit or stash first, or re-run with ALLOW_DIRTY=1."
  fi
  ok "working tree clean"
}

# --- native tier control -----------------------------------------------------

# tier_pid <port> — PID listening on 127.0.0.1:<port>, empty if none.
# Port lookup (not `pkill -f`): a pattern match can kill the deploying shell
# itself when the command line contains the same string.
#
# The trailing `|| true` matters: with `set -e -o pipefail`, grep exiting 1 on
# "nothing listening" would otherwise abort the caller mid-deploy.
tier_pid() {
  ss -tlnp 2>/dev/null | grep ":$1 " | grep -oE 'pid=[0-9]+' | head -1 | cut -d= -f2 || true
}

# tier_stop <port> <label>
tier_stop() {
  local port="$1" label="$2" pid
  pid="$(tier_pid "$port")"
  if [ -z "$pid" ]; then
    info "$label not running"
    return 0
  fi
  info "stopping $label (pid $pid)…"
  kill "$pid"
  local i
  for i in $(seq 1 15); do
    sleep 1
    [ -z "$(tier_pid "$port")" ] && { ok "$label stopped"; return 0; }
  done
  die "$label (pid $pid) did not stop within 15s"
}

# build_snapshot <run-dir> <port>
# Builds the static frontend + the production Go binary from the working tree
# and copies both into the tier's run dir (its own pb_data is untouched).
build_snapshot() {
  local run_dir="$1" port="$2"

  say "── building frontend (PUBLIC_PB_PORT=${port}) ────────────"
  (cd sveltekit && PUBLIC_PB_PORT="$port" pnpm build) || die "frontend build failed"

  say "── building server binary ────────────────────────────────"
  # No `-tags dev`: Automigrate stays OFF on this tier — it only APPLIES the
  # migrations committed in migrations/ (docs/MIGRATIONS.md).
  CGO_ENABLED=0 go build -o /tmp/cart-deploy-server ./cmd/server || die "go build failed"

  say "── copying snapshot → ${run_dir} ─────────────────────────"
  cp /tmp/cart-deploy-server "$run_dir/server"
  rm -rf "$run_dir/pb_public"
  cp -r pb_public "$run_dir/pb_public"
  rm -f /tmp/cart-deploy-server
  ok "snapshot copied ($(ls "$run_dir/pb_public" | wc -l) frontend entries)"
}

# backup_pb_data <run-dir>
backup_pb_data() {
  local run_dir="$1" stamp
  stamp="$(date +%Y%m%d-%H%M%S)"
  [ -d "$run_dir/pb_data" ] || { info "no pb_data yet — nothing to back up"; return 0; }
  cp -r "$run_dir/pb_data" "$run_dir/pb_data.bak-$stamp"
  ok "pb_data backed up → pb_data.bak-$stamp"
}

# tier_start <run-dir> <runner-script> <label>
tier_start() {
  local run_dir="$1" runner="$2" label="$3"
  say "── starting ${label} ─────────────────────────────────────"
  ( cd "$run_dir" && nohup "./$runner" > beta.log 2>&1 & )
  info "launched $runner in $run_dir"
}

# wait_healthy <label> <host-port> <log-file>
# Migrations apply on boot, so this also proves they succeeded.
wait_healthy() {
  local label="$1" port="$2" logfile="$3"
  local url="http://127.0.0.1:${port}/api/health"
  local i

  say "── health check (${url}) ─────────────────"
  for i in $(seq 1 30); do
    if curl -fsS -o /dev/null "$url" 2>/dev/null; then
      ok "${label} healthy on 127.0.0.1:${port}"
      return 0
    fi
    sleep 2
  done

  warn "${label} did not become healthy in ~60s — last logs:"
  tail -40 "$logfile" >&2 || true
  die "deploy failed health check"
}
