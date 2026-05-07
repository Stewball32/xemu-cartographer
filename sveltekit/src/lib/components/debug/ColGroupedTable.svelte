<script lang="ts">
	import JsonTree from '../JsonTree.svelte';
	import AnnotationPill from './AnnotationPill.svelte';
	import type { ColGroup } from './col-grouped-table';

	let {
		rows,
		groups,
		stickyFirst = false,
		emptyMessage = 'no rows',
		annotationPrefix
	}: {
		rows: ReadonlyArray<Record<string, unknown>> | null | undefined;
		groups: ColGroup[];
		stickyFirst?: boolean;
		emptyMessage?: string;
		annotationPrefix?: string;
	} = $props();

	const allCols = $derived(groups.flatMap((g) => g.columns));

	function isScalar(v: unknown): boolean {
		return v === null || v === undefined || typeof v !== 'object';
	}

	function fmt(v: unknown): string {
		if (v === null || v === undefined) return '—';
		if (typeof v === 'number') {
			if (Number.isInteger(v)) return String(v);
			return v.toFixed(4);
		}
		if (typeof v === 'boolean') return v ? '✓' : '·';
		return String(v);
	}
</script>

{#if !rows || rows.length === 0}
	<div class="text-surface-500-400 card preset-tonal p-3 text-sm">{emptyMessage}</div>
{:else}
	<div class="overflow-x-auto">
		<table class="w-full text-xs">
			<thead>
				<tr class="bg-surface-300-700">
					{#each groups as group, gi (gi)}
						<th
							colspan={group.columns.length}
							class="border-surface-400-600 px-2 py-1 text-left font-semibold {gi > 0
								? 'border-l'
								: ''}"
						>
							{group.label}
						</th>
					{/each}
				</tr>
				<tr class="bg-surface-200-800">
					{#each allCols as col, ci (col.key)}
						<th
							class="border-surface-400-600 px-2 py-1 text-left font-medium {ci > 0 &&
							groups.flatMap((g, gi) => g.columns.map(() => gi)).indexOf(ci) === 0
								? 'border-l'
								: ''} {stickyFirst && ci === 0 ? 'sticky left-0 z-10 bg-surface-200-800' : ''}"
						>
							<span class="inline-flex items-center gap-1">
								{#if annotationPrefix}
									<AnnotationPill key="{annotationPrefix}.{col.key}" />
								{/if}
								{col.label ?? col.key}
							</span>
						</th>
					{/each}
				</tr>
			</thead>
			<tbody>
				{#each rows as row, ri (ri)}
					<tr class="border-t border-surface-300-700">
						{#each allCols as col, ci (col.key)}
							<td
								class="px-2 py-1 align-top tabular-nums {stickyFirst && ci === 0
									? 'sticky left-0 bg-surface-50-950'
									: ''}"
							>
								{#if isScalar(row[col.key])}
									<span class="font-mono">{fmt(row[col.key])}</span>
								{:else}
									<JsonTree value={row[col.key]} depth={2} />
								{/if}
							</td>
						{/each}
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}
