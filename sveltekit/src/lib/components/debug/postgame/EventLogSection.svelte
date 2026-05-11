<script lang="ts">
	// Full event log for the just-ended match. Mirrors
	// overview/RecentEvents.svelte but uncapped — every event the runner
	// captured during the match. Wrapped in a scrollable card so the page
	// doesn't grow unbounded when matches have thousands of events.
	import type { Envelope, GamePlayer } from '$lib/types/scraper';
	import EventTile from '../shared/EventTile.svelte';

	let {
		events,
		players = [],
		isTeamGame = false,
		showHeader = true
	}: {
		events: Envelope[];
		players?: GamePlayer[];
		isTeamGame?: boolean;
		showHeader?: boolean;
	} = $props();

	const total = $derived(events.length);

	function colsClass(count: number): string {
		if (count <= 1) return 'grid-cols-1';
		if (count === 2) return 'grid-cols-1 sm:grid-cols-2';
		return 'grid-cols-1 sm:grid-cols-2 md:grid-cols-3';
	}

	// The previous-match event log carries the engine tick of the final tick
	// observed before the runner cleared it. Use the highest tick in the log
	// as a proxy so EventTile's "time ago" footer reads relative to match end
	// rather than against an unrelated runner tick.
	const latestTick = $derived.by(() => {
		let max = 0;
		for (const env of events) {
			if (typeof env.tick === 'number' && env.tick > max) max = env.tick;
		}
		return max > 0 ? max : undefined;
	});
</script>

<section>
	{#if showHeader}
		<div
			class="text-surface-700-200 mb-2 flex items-baseline gap-2 text-xs font-semibold tracking-wide uppercase"
		>
			Event log
			<span class="text-surface-500-400 font-normal normal-case">
				{#if total === 0}
					none
				{:else}
					{total} {total === 1 ? 'event' : 'events'}
				{/if}
			</span>
		</div>
	{/if}
	{#if events.length === 0}
		<div class="text-surface-500-400 card preset-tonal p-3 text-sm">
			no events captured for the previous match
		</div>
	{:else}
		<div class="max-h-[60vh] overflow-y-auto card preset-tonal p-2">
			<div class="grid gap-2 {colsClass(events.length)}">
				{#each events as env, i (env.tick + ':' + i)}
					<EventTile {env} {players} {isTeamGame} {latestTick} />
				{/each}
			</div>
		</div>
	{/if}
</section>
