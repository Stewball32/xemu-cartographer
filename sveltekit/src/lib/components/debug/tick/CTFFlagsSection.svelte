<script lang="ts">
	// Renders TickPayload.ctf_flags — one row per flag with team / pose / carrier
	// / status. Only populated on CTF gametypes; the empty state lets the user
	// confirm that the scraper *did* return an empty list rather than skipping
	// the read.
	import type { ColGroup } from '../shared/col-grouped-table';
	import ColGroupedTable from '../shared/ColGroupedTable.svelte';
	import type { TickCTFFlag } from '$lib/types/scraper';
	import { teamLabel } from '../shared/util';

	let {
		flags,
		showHeader = true
	}: {
		flags: TickCTFFlag[] | null | undefined;
		showHeader?: boolean;
	} = $props();

	const groups: ColGroup[] = [
		{
			label: 'Flag',
			columns: [
				{ key: 'team_label', label: 'team' },
				{ key: 'status', label: 'status' },
				{ key: 'carrier_index', label: 'carrier' }
			]
		},
		{
			label: 'Position',
			columns: [
				{ key: 'x', label: 'x' },
				{ key: 'y', label: 'y' },
				{ key: 'z', label: 'z' }
			]
		}
	];

	const rows = $derived(
		(flags ?? []).map((f) => ({
			team_label: `${teamLabel(f.team)} (${f.team})`,
			status: f.status,
			carrier_index: f.carrier_index ?? '—',
			x: f.x,
			y: f.y,
			z: f.z
		}))
	);
</script>

<section>
	{#if showHeader}
		<div class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			CTF flags ({rows.length})
		</div>
	{/if}
	<ColGroupedTable {rows} {groups} emptyMessage="no CTF flags in this tick (non-CTF gametype?)" />
</section>
