#!/usr/bin/env bash
# Shared helpers for migrate-squash.sh / migrate-verify.sh. Not executable alone.
#
# Migration classification — the whole squash workflow rests on this split:
#
#   RELEASE-line : *_collections_snapshot.go  (the original baseline)
#                  *_release_*.go             (squashed per-release migrations)
#                  → these are what main/prod carry, one per approved version
#
#   GRANULAR     : everything else in migrations/ (excluding doc.go)
#                  → Automigrate output and `migrate create` files; dev/test only,
#                    squashed away before merging to main

list_releases() {
  find migrations -maxdepth 1 -name '*.go' \
    \( -name '*_collections_snapshot.go' -o -name '*_release_*.go' \) \
    2>/dev/null | sort
}

list_granular() {
  find migrations -maxdepth 1 -name '*.go' \
    ! -name 'doc.go' \
    ! -name '*_collections_snapshot.go' \
    ! -name '*_release_*.go' \
    2>/dev/null | sort
}

# seed_data_dir <src-pb_data|""> <dest>
# Empty src = start from a brand-new database.
seed_data_dir() {
  local src="$1" dest="$2"
  mkdir -p "$dest"
  if [ -n "$src" ]; then
    [ -d "$src" ] || { printf '✗ --from directory not found: %s\n' "$src" >&2; exit 1; }
    cp -a "$src/." "$dest/"
  fi
}

# apply_migrations <binary> <data-dir>
# Runs pending migrations. `migrate up` applies and exits — no server needed.
apply_migrations() {
  local bin="$1" data="$2"
  "$bin" migrate up --dir "$data" >/dev/null 2>&1
}

# dump_schema <data-dir> <out-file>
# Normalized, stable dump of the collection schema for comparison. Excludes the
# volatile created/updated timestamps; keeps ids, fields, indexes and API rules,
# so a difference in any of those is caught.
dump_schema() {
  local data="$1" out="$2"
  sqlite3 "$data/data.db" \
    "SELECT id||'|'||name||'|'||type||'|'||
            COALESCE(fields,'')||'|'||COALESCE(indexes,'')||'|'||
            COALESCE(listRule,'~')||'|'||COALESCE(viewRule,'~')||'|'||
            COALESCE(createRule,'~')||'|'||COALESCE(updateRule,'~')||'|'||
            COALESCE(deleteRule,'~')
     FROM _collections ORDER BY name;" > "$out"
}
