<script lang="ts">
	// Static power-item spawn table — overshield, camo, weapon-pickups etc.
	// Renders via shared/ColGroupedTable. `tag` is the scenario tag path which
	// can be long, so it leans on the table's horizontal-scroll affordance
	// rather than trying to truncate.

	import type { PowerItemSpawn } from '$lib/types/scraper';
	import ColGroupedTable from '../shared/ColGroupedTable.svelte';
	import type { ColGroup } from '../shared/col-grouped-table';

	let {
		spawns,
		showHeader = true
	}: {
		spawns: PowerItemSpawn[];
		showHeader?: boolean;
	} = $props();

	const groups: ColGroup[] = [
		{
			label: 'Power-item spawns',
			columns: [
				{ key: 'spawn_id', label: 'id' },
				{ key: 'tag' },
				{ key: 'spawn_interval_ticks', label: 'interval' },
				{ key: 'x' },
				{ key: 'y' },
				{ key: 'z' }
			]
		}
	];
</script>

<section>
	{#if showHeader}
		<div class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			Power items ({spawns.length})
		</div>
	{/if}
	<ColGroupedTable
		rows={spawns as unknown as Record<string, unknown>[]}
		{groups}
		stickyFirst
		emptyMessage="no power-item spawns"
	/>
</section>
