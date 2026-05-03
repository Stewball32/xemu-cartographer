<script lang="ts">
	import JsonTree from '../JsonTree.svelte';
	import type { ColGroup } from './col-grouped-table';

	let {
		rows,
		groups,
		stickyFirst = false,
		emptyMessage = 'no rows'
	}: {
		rows: ReadonlyArray<Record<string, unknown>> | null | undefined;
		groups: ColGroup[];
		stickyFirst?: boolean;
		emptyMessage?: string;
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
	<div class="card preset-tonal text-surface-500-400 p-3 text-sm">{emptyMessage}</div>
{:else}
	<div class="overflow-x-auto">
		<table class="w-full text-xs">
			<thead>
				<tr class="bg-surface-300-700">
					{#each groups as group, gi}
						<th
							colspan={group.columns.length}
							class="border-surface-400-600 px-2 py-1 text-left font-semibold {gi >
							0
								? 'border-l'
								: ''}"
						>
							{group.label}
						</th>
					{/each}
				</tr>
				<tr class="bg-surface-200-800">
					{#each allCols as col, ci}
						<th
							class="border-surface-400-600 px-2 py-1 text-left font-medium {ci > 0 && groups.flatMap((g, gi) => g.columns.map(() => gi)).indexOf(ci) === 0 ? 'border-l' : ''} {stickyFirst && ci === 0 ? 'bg-surface-200-800 sticky left-0 z-10' : ''}"
						>
							{col.label ?? col.key}
						</th>
					{/each}
				</tr>
			</thead>
			<tbody>
				{#each rows as row}
					<tr class="border-surface-300-700 border-t">
						{#each allCols as col, ci}
							<td
								class="px-2 py-1 align-top tabular-nums {stickyFirst && ci === 0 ? 'bg-surface-50-950 sticky left-0' : ''}"
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
