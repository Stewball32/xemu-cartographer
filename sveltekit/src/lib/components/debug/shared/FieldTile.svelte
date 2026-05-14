<script lang="ts" module>
	// FieldTile — labelled card combining StatTile's look with KvGrid's
	// wire-source visibility. Each tile has exactly one label at the top
	// (the human-readable name for the field or grouping), then one or
	// more rows under it. Each row is laid out left-to-right:
	//   - source  (left — the JSON / wire variable name, monospace, dim, so
	//              the operator can trace what they see back to the payload)
	//   - value   (right — the formatted display, bold mono — defaults to
	//              the raw wire value so debugging stays faithful)
	//   - context (right, tucked under the value — optional smaller / dimmer
	//              human reading of the raw value, e.g. `is_team_game=true →
	//              "team"`, `time_limit_ticks=18000 → "10:00"`. Use it when
	//              the raw value is the source of truth but a derived view
	//              aids skimming. Leave undefined when self-explanatory.)
	//
	// Single-row tiles use the singular `source` + `value` / `display`
	// shorthand. Multi-row tiles pass `rows` so related wire fields can
	// share one label without inventing a synthetic header — e.g. a
	// "Gametype" tile that surfaces both `gametype` and `is_team_game`.
	// Rows are separated by a thin divider.
	//
	// For dense uniform key/value lists where every key IS the wire key
	// (no separate human label), KvGrid / KvCard remain the right choice.

	export type FieldTileStatusKind = 'on' | 'off' | 'warn' | 'info' | 'neutral' | 'none';

	export type FieldRow = {
		source: string;
		value?: unknown;
		display?: string;
		context?: string;
		annotationKey?: string;
		title?: string;
		statusKind?: FieldTileStatusKind;
		format?: (v: unknown) => string;
	};
</script>

<script lang="ts">
	import AnnotationPill from './AnnotationPill.svelte';

	let {
		label,
		title,
		annotationKey,
		// Single-row shorthand — equivalent to `rows={[{ source, value, ... }]}`.
		source,
		value,
		display,
		context,
		statusKind = 'none',
		format,
		// Multi-row escape hatch — takes precedence over the singular props.
		rows
	}: {
		label: string;
		title?: string;
		annotationKey?: string;
		source?: string;
		value?: unknown;
		display?: string;
		context?: string;
		statusKind?: FieldTileStatusKind;
		format?: (v: unknown) => string;
		rows?: FieldRow[];
	} = $props();

	function defaultFmt(v: unknown): string {
		if (v === null || v === undefined) return '—';
		if (typeof v === 'number') return Number.isInteger(v) ? String(v) : v.toFixed(4);
		if (typeof v === 'boolean') return v ? 'true' : 'false';
		return String(v);
	}

	function valueColorFor(k: FieldTileStatusKind): string {
		switch (k) {
			case 'on':
				return 'text-success-500';
			case 'off':
				return 'text-error-500';
			case 'warn':
				return 'text-warning-500';
			case 'info':
				return 'text-secondary-500';
			case 'neutral':
				return 'text-surface-500-400';
			default:
				return 'text-surface-950-50';
		}
	}

	const effectiveRows = $derived<FieldRow[]>(
		rows ?? (source !== undefined ? [{ source, value, display, context, statusKind, format }] : [])
	);
</script>

<div {title} class="flex h-full min-w-0 flex-col card preset-tonal p-2">
	<div class="mb-1 flex min-w-0 items-baseline gap-1.5">
		<span
			class="text-surface-700-200 min-w-0 flex-1 truncate text-[10px] font-semibold tracking-wide uppercase"
		>
			{label}
		</span>
		{#if annotationKey}
			<AnnotationPill key={annotationKey} />
		{/if}
	</div>

	{#each effectiveRows as row, i (i)}
		{@const rowDisplay = row.display ?? (row.format ?? defaultFmt)(row.value)}
		{@const rowColor = valueColorFor(row.statusKind ?? 'none')}
		<div
			title={row.title}
			class="flex min-w-0 items-baseline justify-between gap-3 py-1 first:pt-0 last:pb-0 {i > 0
				? 'mt-1 border-t border-surface-300-700/40 pt-1.5'
				: ''}"
		>
			<span class="flex min-w-0 flex-1 items-baseline gap-1.5">
				<code class="text-surface-500-400 min-w-0 truncate font-mono text-[10px]">
					{row.source}
				</code>
				{#if row.annotationKey}
					<AnnotationPill key={row.annotationKey} />
				{/if}
			</span>
			<span class="flex min-w-0 flex-col items-end">
				<span class="min-w-0 truncate font-mono text-sm font-bold tabular-nums {rowColor}">
					{rowDisplay}
				</span>
				{#if row.context}
					<span class="text-surface-500-400 min-w-0 truncate font-mono text-[10px]">
						{row.context}
					</span>
				{/if}
			</span>
		</div>
	{/each}
</div>
