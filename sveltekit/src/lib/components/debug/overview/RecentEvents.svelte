<script lang="ts">
	import type { Envelope } from '$lib/types/scraper';
	import type { EventRosterRef } from '../shared/events-vm';
	import EventTile from './EventTile.svelte';

	let {
		events,
		players = [],
		isTeamGame = false,
		latestTick,
		limit = 10,
		showHeader = true
	}: {
		events: Envelope[];
		players?: EventRosterRef[];
		isTeamGame?: boolean;
		latestTick?: number | undefined;
		limit?: number;
		showHeader?: boolean;
	} = $props();

	const recent = $derived(events.slice(0, limit));
	const total = $derived(events.length);
	const shown = $derived(recent.length);

	// Tiles now carry icon + summary + footer (4-5 lines tall), so cap at 3
	// columns to keep summary lines readable. Mobile is always 1 col.
	function colsClass(count: number): string {
		if (count <= 1) return 'grid-cols-1';
		if (count === 2) return 'grid-cols-1 sm:grid-cols-2';
		return 'grid-cols-1 sm:grid-cols-2 md:grid-cols-3';
	}
</script>

<section>
	{#if showHeader}
		<div
			class="text-surface-700-200 mb-2 flex items-baseline gap-2 text-xs font-semibold tracking-wide uppercase"
		>
			Recent events
			<span class="text-surface-500-400 font-normal normal-case">
				{#if total === 0}
					none yet
				{:else}
					latest {shown} of {total}
				{/if}
			</span>
		</div>
	{/if}
	{#if recent.length === 0}
		<div class="text-surface-500-400 card preset-tonal p-3 text-sm">no events yet</div>
	{:else}
		<div class="grid gap-2 {colsClass(recent.length)}">
			{#each recent as env, i (env.tick + ':' + i)}
				<EventTile {env} {players} {isTeamGame} {latestTick} />
			{/each}
		</div>
		{#if total > limit}
			<div class="mt-2 text-right text-xs">
				<a class="text-primary-500 hover:underline" href="#events">
					View all ({total}) →
				</a>
			</div>
		{/if}
	{/if}
</section>
