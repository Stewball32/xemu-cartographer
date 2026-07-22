#!/usr/bin/env bash
#
# migrate-verify.sh — prove a squashed release migration is equivalent to the
# granular migrations it replaced.
#
# THIS IS THE PROVE-BEFORE-PROD CHECK. Run it after migrate-squash.sh and before
# merging to main — ideally with --from pointing at a COPY of prod's pb_data, so
# you are verifying the exact upgrade path prod will take.
#
# How it works — two independent runs from the SAME starting database:
#
#   A (granular) : starting data + release-line files + the archived granular files
#   B (squashed) : starting data + release-line files + the new release migration
#
# Both end schemas are dumped and compared. They must be byte-identical.
#
# Usage:
#   ./scripts/migrate-verify.sh [--from <pb_data-dir>] [--version <v0.3.0>]
#
#   --from     start both runs from a copy of this pb_data (STRONGLY recommended:
#              use a copy of prod). Omitted = compare from an empty database.
#   --version  which .migrate-archive/<version>/ holds the granular files
#              (default: the most recently modified archive directory)
set -euo pipefail
cd "$(dirname "$0")/.."

FROM=""; VERSION=""
die() { printf '✗ %s\n' "$*" >&2; exit 1; }
say() { printf '%s\n' "$*"; }
ok()  { printf '✓ %s\n' "$*"; }

while [ $# -gt 0 ]; do
  case "$1" in
    --from)    FROM="${2:-}"; shift 2 ;;
    --version) VERSION="${2:-}"; shift 2 ;;
    -h|--help) sed -n '2,25p' "$0"; exit 0 ;;
    *)         die "unknown argument: $1" ;;
  esac
done

# shellcheck source=scripts/migrate-lib.sh
. scripts/migrate-lib.sh

command -v sqlite3 >/dev/null 2>&1 || die "sqlite3 is required to compare schemas"

# Locate the archived granular set produced by migrate-squash.sh.
if [ -z "$VERSION" ]; then
  ARCHIVE="$(find .migrate-archive -maxdepth 1 -mindepth 1 -type d 2>/dev/null | sort | tail -1)"
else
  ARCHIVE=".migrate-archive/${VERSION}"
fi
[ -n "$ARCHIVE" ] && [ -d "$ARCHIVE" ] \
  || die "no archived granular migrations found — run ./scripts/migrate-squash.sh first"

mapfile -t ARCHIVED < <(find "$ARCHIVE" -maxdepth 1 -name '*.go' | sort)
[ "${#ARCHIVED[@]}" -gt 0 ] || die "archive $ARCHIVE contains no migration files"

# The newest release-line file is the squashed migration under test.
RELEASE="$(list_releases | tail -1)"
[ -n "$RELEASE" ] || die "no release migration found in migrations/"

say "── verify squash ─────────────────────────────────────────"
say "  squashed file : $(basename "$RELEASE")"
say "  granular set  : ${#ARCHIVED[@]} file(s) from $ARCHIVE"
say "  starting data : ${FROM:-<empty database>}"
say "──────────────────────────────────────────────────────────"

WORK="$(mktemp -d)"; trap 'rm -rf "$WORK"' EXIT

# --- B: squashed (the current working tree) ---------------------------------
say "building B (squashed)…"
BIN_B="$WORK/server-squashed"
go build -o "$BIN_B" ./cmd/server || die "go build failed (squashed tree)"

# --- A: granular (repo copy with the archive restored, release removed) ------
say "building A (granular)…"
TREE_A="$WORK/tree-granular"
mkdir -p "$TREE_A"
# Copy the repo without the heavy/irrelevant dirs.
tar -cf - --exclude=.git --exclude=node_modules --exclude=pb_data \
    --exclude=pb_public --exclude=tmp --exclude=.migrate-archive . \
  | tar -xf - -C "$TREE_A"
rm -f "$TREE_A/$RELEASE"                       # drop the squashed migration
cp "${ARCHIVED[@]}" "$TREE_A/migrations/"      # restore the granular ones
BIN_A="$WORK/server-granular"
( cd "$TREE_A" && go build -o "$BIN_A" ./cmd/server ) || die "go build failed (granular tree)"

# --- apply both to identical starting data ----------------------------------
say "applying A (granular)…"
DATA_A="$WORK/data-a"; seed_data_dir "$FROM" "$DATA_A"
apply_migrations "$BIN_A" "$DATA_A" || die "granular migrations failed to apply"

say "applying B (squashed)…"
DATA_B="$WORK/data-b"; seed_data_dir "$FROM" "$DATA_B"
apply_migrations "$BIN_B" "$DATA_B" || die "squashed migration failed to apply"

# --- compare -----------------------------------------------------------------
dump_schema "$DATA_A" "$WORK/schema-a.txt"
dump_schema "$DATA_B" "$WORK/schema-b.txt"

say
if diff -q "$WORK/schema-a.txt" "$WORK/schema-b.txt" >/dev/null; then
  ok "schemas are IDENTICAL — the squash is equivalent"
  say "  collections compared: $(wc -l < "$WORK/schema-a.txt")"
  say
  say "Safe to merge. Prod will apply $(basename "$RELEASE") on its next deploy."
  exit 0
fi

printf '✗ SCHEMAS DIFFER — do NOT merge this squash\n\n' >&2
say "granular (A) vs squashed (B):" >&2
diff <(tr '|' '\n' < "$WORK/schema-a.txt") <(tr '|' '\n' < "$WORK/schema-b.txt") \
  | head -40 >&2
say >&2
say "Common cause: the granular set DELETED a collection. A snapshot imports with" >&2
say "deleteMissing=false, so it never removes one. Re-run the squash with" >&2
say "--delete-missing (destructive: it drops collections absent from the snapshot)." >&2
exit 1
