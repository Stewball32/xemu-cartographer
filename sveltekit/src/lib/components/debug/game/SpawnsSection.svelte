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

	// Every StaticPlayerSpawn field gets a column — this is a debug surface,
	// hiding wire-level fields makes the scraper harder to verify. Grouped
	// into spawn-id / position / scenario-gametype flags so the wide table
	// stays scannable horizontally.
	const groups: ColGroup[] = [
		{
			label: 'Spawn',
			columns: [
				{ key: 'index', label: 'idx' },
				{ key: 'team_index', label: 'team' },
				{ key: 'bsp_index', label: 'bsp' }
			]
		},
		{
			label: 'Position',
			columns: [{ key: 'x' }, { key: 'y' }, { key: 'z' }, { key: 'facing' }]
		},
		{
			label: 'Gametype flags',
			columns: [
				{ key: 'unk_0' },
				{ key: 'gametype_0' },
				{ key: 'gametype_1' },
				{ key: 'gametype_2' },
				{ key: 'gametype_3' }
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
