<script lang="ts">
	// Static player spawns table — scenario-derived spawn points for the
	// current map. Renders via shared/ColGroupedTable with a single column group
	// to keep the table compact while still surfacing every field of
	// StaticPlayerSpawn that maps to a 3D location + scenario tag.

	import type { StaticPlayerSpawn } from '$lib/types/scraper';
	import ColGroupedTable from '../shared/ColGroupedTable.svelte';
	import type { ColGroup } from '../shared/col-grouped-table';

	let {
		spawns,
		showHeader = true
	}: {
		spawns: StaticPlayerSpawn[];
		showHeader?: boolean;
	} = $props();

	const groups: ColGroup[] = [
		{
			label: 'Player spawns',
			columns: [
				{ key: 'index', label: 'idx' },
				{ key: 'team_index', label: 'team' },
				{ key: 'bsp_index', label: 'bsp' },
				{ key: 'x' },
				{ key: 'y' },
				{ key: 'z' },
				{ key: 'facing' }
			]
		}
	];
</script>

<section>
	{#if showHeader}
		<div class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			Player spawns ({spawns.length})
		</div>
	{/if}
	<ColGroupedTable
		rows={spawns as unknown as Record<string, unknown>[]}
		{groups}
		stickyFirst
		emptyMessage="no player spawns"
	/>
</section>
