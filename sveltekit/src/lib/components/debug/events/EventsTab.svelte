<script lang="ts">
	// Events tab — interactive type-chip filter (doubles as the frequency
	// display) and scrollable feed of EventTile cards. Empty selectedTypes =
	// show all.

	import { SvelteSet } from 'svelte/reactivity';
	import { scraperWSV2 } from '$lib/stores/scraper-ws-v2.svelte';
	import { useDebugContext } from '../context.js';
	import { buildEventsTabVm } from './events-tab-vm';
	import TypeFilter from './TypeFilter.svelte';
	import EventFeed from './EventFeed.svelte';

	let { name }: { name: string } = $props();

	// SvelteSet (not Set) so the parent's $derived sees membership changes
	// without identity swaps.
	let selectedTypes = $state(new SvelteSet<string>());

	const ctx = useDebugContext();

	const vm = $derived.by(() => buildEventsTabVm(name, scraperWSV2, selectedTypes));

	// Player chips: roster comes from the v2 game class; fall back to the
	// REST inspect snapshot so chips render before the first game envelope.
	const v2Game = $derived(scraperWSV2.game[name] ?? null);
	const players = $derived(v2Game?.players ?? ctx.inspect?.game_data?.players ?? []);
	const isTeamGame = $derived(
		v2Game?.config?.is_team_game ?? ctx.inspect?.game_data?.is_team_game === true
	);
	const latestTick = $derived(scraperWSV2.engineTick[name] ?? ctx.inspect?.tick);
</script>

<div class="flex flex-col gap-4">
	<section>
		<div
			class="text-surface-700-200 mb-2 flex items-baseline gap-2 text-xs font-semibold tracking-wide uppercase"
		>
			By type
			<span class="text-surface-500-400 font-normal normal-case">
				{#if vm.typeBuckets.length === 0}
					none yet
				{:else}
					{vm.typeBuckets.length} type{vm.typeBuckets.length === 1 ? '' : 's'}
					{#if selectedTypes.size > 0}
						· {selectedTypes.size} selected
					{:else}
						· showing all
					{/if}
				{/if}
			</span>
		</div>
		{#if vm.typeBuckets.length === 0}
			<div class="text-surface-500-400 text-xs">no events to summarise</div>
		{:else}
			<TypeFilter typeBuckets={vm.typeBuckets} {selectedTypes} />
		{/if}
	</section>

	<EventFeed
		events={vm.filteredEvents}
		{players}
		{isTeamGame}
		{latestTick}
		totalCount={vm.totalCount}
		filterActive={vm.filterActive}
	/>
</div>
