<script lang="ts">
	// Renders TickPayload.power_items — runtime status of every power-item spawn
	// (held / world / respawning). Pairs with the Game tab's PowerItemsSection,
	// which shows the static spawn config.
	import type { ColGroup } from '../shared/col-grouped-table';
	import ColGroupedTable from '../shared/ColGroupedTable.svelte';
	import type { PowerItemStatus } from '$lib/types/scraper';

	let {
		powerItems,
		showHeader = true
	}: {
		powerItems: PowerItemStatus[] | null | undefined;
		showHeader?: boolean;
	} = $props();

	const groups: ColGroup[] = [
		{
			label: 'Spawn',
			columns: [
				{ key: 'spawn_id', label: 'id' },
				{ key: 'status', label: 'status' }
			]
		},
		{
			label: 'World position',
			columns: [
				{ key: 'x', label: 'x' },
				{ key: 'y', label: 'y' },
				{ key: 'z', label: 'z' }
			]
		},
		{
			label: 'Live',
			columns: [
				{ key: 'held_by', label: 'held by' },
				{ key: 'respawn_in_ticks', label: 'respawn' }
			]
		}
	];

	const rows = $derived(
		(powerItems ?? []).map((p) => ({
			spawn_id: p.spawn_id,
			status: p.status,
			x: p.world_pos?.x ?? '—',
			y: p.world_pos?.y ?? '—',
			z: p.world_pos?.z ?? '—',
			held_by: p.held_by ?? '—',
			respawn_in_ticks: p.respawn_in_ticks ?? '—'
		}))
	);
</script>

<section>
	{#if showHeader}
		<div class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			Power items ({rows.length})
		</div>
	{/if}
	<ColGroupedTable {rows} {groups} emptyMessage="no live power-item statuses in this tick" />
</section>
