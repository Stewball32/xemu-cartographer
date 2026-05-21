<script lang="ts">
	// Scrollable feed of EventTile cards (newest-first). With the upstream
	// 100-event cap there's no need for windowed/virtual scrolling yet; if
	// the cap grows materially, swap the {#each} for a windowed list.

	import type { Envelope } from '$lib/types/scraper';
	import EventTile from '../shared/EventTile.svelte';
	import type { EventRosterRef } from '../shared/events-vm';

	let {
		events,
		players,
		isTeamGame,
		latestTick,
		totalCount,
		filterActive
	}: {
		events: Envelope[];
		players: EventRosterRef[];
		isTeamGame: boolean;
		latestTick: number | undefined;
		totalCount: number;
		filterActive: boolean;
	} = $props();
</script>

<section>
	<div
		class="text-surface-700-200 mb-2 flex items-baseline gap-2 text-xs font-semibold tracking-wide uppercase"
	>
		Event feed
		<span class="text-surface-500-400 font-normal normal-case">
			{#if totalCount === 0}
				none yet
			{:else if filterActive}
				{events.length} of {totalCount} shown
			{:else}
				{events.length} total
			{/if}
		</span>
	</div>
	{#if events.length === 0}
		<div class="text-surface-500-400 card preset-tonal p-3 text-sm">
			{#if totalCount === 0}
				no events yet — feed populates while the scraper is in the live phase
			{:else}
				no events match the current filter
			{/if}
		</div>
	{:else}
		<div
			class="grid max-h-[70vh] grid-cols-1 gap-2 overflow-y-auto pr-1 sm:grid-cols-2 md:grid-cols-3"
		>
			{#each events as env, i (env.tick + ':' + i)}
				<EventTile {env} {players} {isTeamGame} {latestTick} />
			{/each}
		</div>
	{/if}
</section>
