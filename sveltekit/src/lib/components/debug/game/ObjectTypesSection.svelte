<script lang="ts">
	// Renders GameData.object_types — the engine's per-type registration table.
	// Each entry is a (type_index, name, datum_size) tuple read at match start.
	// One row per type so the user can verify the registration scrape produced
	// every expected entry.
	import type { ColGroup } from '../shared/col-grouped-table';
	import ColGroupedTable from '../shared/ColGroupedTable.svelte';
	import type { StaticObjectType } from '$lib/types/scraper';

	let {
		objectTypes,
		showHeader = true
	}: {
		objectTypes: StaticObjectType[];
		showHeader?: boolean;
	} = $props();

	const groups: ColGroup[] = [
		{
			label: 'Object',
			columns: [
				{ key: 'type_index', label: 'idx' },
				{ key: 'name', label: 'name' },
				{ key: 'datum_size', label: 'datum size' }
			]
		}
	];

	const rows = $derived(
		objectTypes.map((o) => ({
			type_index: o.type_index,
			name: o.name || '—',
			datum_size: o.datum_size
		}))
	);
</script>

<section>
	{#if showHeader}
		<div class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			Object types ({objectTypes.length})
		</div>
	{/if}
	<ColGroupedTable {rows} {groups} emptyMessage="no object types in current match data" />
</section>
