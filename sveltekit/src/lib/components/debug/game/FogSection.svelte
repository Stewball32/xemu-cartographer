<script lang="ts">
	// Static fog parameters — scenario-derived RGB color and density / distance
	// envelope. Renders via shared/KvGrid as a labelled key/value list because
	// there's no useful table shape (it's a fixed flat record).

	import type { StaticFog } from '$lib/types/scraper';
	import KvGrid, { type KvField } from '../shared/KvGrid.svelte';

	let {
		fog,
		showHeader = true
	}: {
		fog: StaticFog | null;
		showHeader?: boolean;
	} = $props();

	const fields = $derived<KvField[] | null>(
		fog
			? [
					{ key: 'color_r', value: fog.color_r },
					{ key: 'color_g', value: fog.color_g },
					{ key: 'color_b', value: fog.color_b },
					{ key: 'max_density', value: fog.max_density },
					{ key: 'atmo_min_dist', value: fog.atmo_min_dist },
					{ key: 'atmo_max_dist', value: fog.atmo_max_dist }
				]
			: null
	);
</script>

<section>
	{#if showHeader}
		<div class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">Fog</div>
	{/if}
	{#if !fog}
		<div class="text-surface-500-400 card preset-tonal p-3 text-sm">no fog data</div>
	{:else}
		<div class="card preset-tonal p-3">
			<KvGrid {fields} />
		</div>
	{/if}
</section>
