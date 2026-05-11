<script lang="ts">
	// Network-machine list (system-link only — emits one entry per Xbox in the
	// system-link session). Rendered as a compact table; in splitscreen-only
	// matches the list is empty and we surface that as a friendly empty state.

	import type { GameMachine } from '$lib/types/scraper';
	import ColGroupedTable from '../shared/ColGroupedTable.svelte';
	import type { ColGroup } from '../shared/col-grouped-table';

	let {
		machines,
		showHeader = true
	}: {
		machines: GameMachine[];
		showHeader?: boolean;
	} = $props();

	const groups: ColGroup[] = [
		{
			label: 'Machines',
			columns: [{ key: 'index', label: 'idx' }, { key: 'name' }]
		}
	];
</script>

<section>
	{#if showHeader}
		<div class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			Machines ({machines.length})
		</div>
	{/if}
	<ColGroupedTable
		rows={machines as unknown as Record<string, unknown>[]}
		{groups}
		stickyFirst
		emptyMessage="no network machines (splitscreen-only match)"
	/>
</section>
