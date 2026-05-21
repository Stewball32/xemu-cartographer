# Tables

The canonical table component for this project is `<DataTable>` at
`src/lib/components/ui/DataTable.svelte`. Use it for any list of records the
user might scan or sort. This doc codifies the conventions so tables stay
consistent as the app grows.

## When to use what

| Scenario                                                  | Component                        |
| --------------------------------------------------------- | -------------------------------- |
| Any feature/admin page list (records, statuses, history)  | `DataTable`                      |
| Ad-hoc JSON in debug tabs (raw envelopes, probe payloads) | `JsonTree`                       |
| Key/value pairs (config, single-record fields)            | `KvCard` / `KvGrid`              |
| Anything else (custom layout, dual-mode responsive)       | bespoke `<table>` — needs reason |

If you reach for a bespoke `<table>`, leave a comment at the top of the file
explaining what `DataTable` couldn't do (e.g. team-grouped rows in
`RosterList`, podium semantics in `ScoreHero`). Future contributors should
know it was a deliberate choice.

## Required props for record lists

When rendering a list of records via `DataTable`, treat these as required:

- **`defaultSort`** — mandatory whenever source order isn't deterministic.
  HTTP responses, websocket envelopes, and Go-marshaled data all qualify.
  Pick a sensible primary column (usually `name` or `created`) so the table
  doesn't shuffle on refresh.
- **`secondarySort`** — strongly recommended; breaks ties between rows that
  share the primary sort key, so equal rows don't reorder across renders.
- **`emptyMessage`** — always set. The default `"no rows"` is fine for debug
  but feature pages should say something useful (e.g. `"No containers yet.
Create one to get started."`).
- **`loading`** — pass `true` while the first fetch is in flight; the table
  renders `loadingRows` skeleton rows. Don't show a separate spinner above
  the table.

## Density

- `density="compact"` (default) — `text-xs px-2 py-1`. Use for debug pages
  where information density matters more than scanability.
- `density="comfortable"` — `text-sm px-4 py-2`. Use for admin and feature
  pages where the table is the page's primary content.

## Common cell patterns

### Status badges + priority sort

Use a `cell` snippet for the badge and a `comparator` for the priority
order:

```svelte
{
  key: 'status',
  comparator: (a, b) => priority(a.status) - priority(b.status),
  cell: statusBadge
}
```

`priority` returns a number per status (e.g. `running=0, starting=1, stopped=2,
error=3, loading=4`).

### Action buttons

Action columns are not sortable and right-aligned:

```svelte
{ key: 'actions', sortable: false, align: 'right', cell: actions }
```

### Relative timestamps

Render-formatted text in the cell, but sort on the underlying epoch:

```svelte
{
  key: 'created',
  sortAccessor: (r) => Date.parse(r.created),
  cell: relativeDate
}
```

### Two-line cells (primary + secondary text)

Use a `cell` snippet and pick whichever line drives sort:

```svelte
{
  key: 'game_xbox',
  sortAccessor: (r) => r.title ?? '',
  cell: gameXboxCell
}
```

### Row click → detail page

Set `onRowClick` and the table makes rows keyboard-accessible automatically:

```svelte
<DataTable {rows} {groups} onRowClick={(r) => goto(`/containers/${r.name}/`)} />
```

## Order stability

The single biggest source of "tables shuffle on refresh" is non-deterministic
source order. Apply these rules:

- **DataTable lists** — always pass `defaultSort`. The sort + secondary
  tiebreak + original-index tiebreak combine to keep rows stable across
  renders even when the source array reorders.
- **Raw KV maps** (`KvCard` / `KvGrid`) — render keys alphabetically by
  default. `KvCard` accepts `entriesSort="none"` for callers that need to
  preserve insertion order intentionally.
- **Intentionally-ordered slices** (e.g. team rosters sorted server-side) —
  keep source order, document why in a comment, and prefer a server-side
  contract that guarantees the order rather than relying on client sort.

The Go backend's `encoding/json` already alphabetizes string-keyed maps when
marshaling, so frontend `Object.entries(...)` over a JSON-parsed map
preserves alphabetical order. The render-side defensive sort guards against
any future marshaller swap and against in-Svelte mutations.

## Out of scope (future work)

- **Filtering** — no built-in filter UI. Pre-filter `rows` in the parent
  component for now; we'll add a generic filter slot once a real use case
  shows up.
- **Pagination** — when needed, pair with Skeleton's `<Pagination>` from
  `@skeletonlabs/skeleton-svelte`. The DataTable itself stays paging-
  agnostic; pass it the current page's slice.
- **Virtualization** — not needed at current data volumes. Revisit if a
  table ever needs > ~500 rows.
- **Multi-column sort UI** — header click is single-column tri-state
  (asc → desc → none). Multi-column compound sort can be added later via
  the existing `secondarySort` prop or by extending `SortState` to a list.
- **ColGroupedTable absorption** — the older `ColGroupedTable` in
  `debug/shared/` predates `DataTable` and has a narrow physics-tick use
  case. Leave it for now; fold into `DataTable` once a `groupBy` extension
  is designed.
