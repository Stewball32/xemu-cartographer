<script lang="ts">
	// Events tab — frequency strip + multi-select type filter + scrollable
	// feed of EventTile cards. Empty selectedTypes = show all.

	import { SvelteSet } from 'svelte/reactivity';
	import { scraperWS } from '$lib/stores/scraper-ws.svelte';
	import { useDebugContext } from '../context.js';
	import { buildEventsTabVm } from './events-tab-vm';
	import FrequencyStrip from './FrequencyStrip.svelte';
	import TypeFilter from './TypeFilter.svelte';
	import EventFeed from './EventFeed.svelte';

	let { name }: { name: string } = $props();

	// SvelteSet (not Set) so the parent's $derived sees membership changes
	// without identity swaps.
	let selectedTypes = $state(new SvelteSet<string>());

	const ctx = useDebugContext();

	const vm = $derived.by(() => buildEventsTabVm(name, scraperWS, selectedTypes));

	// gameData fallback to the inspect snapshot so player chips render
	// before the first WS current_state arrives.
	const gameData = $derived(scraperWS.gameData[name] ?? ctx.inspect?.game_data ?? null);
	const players = $derived(gameData?.players ?? []);
	const isTeamGame = $derived(gameData?.is_team_game === true);
	const latestTick = $derived(scraperWS.tickNumbers[name] ?? ctx.inspect?.tick);
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
				{/if}
			</span>
		</div>
		<FrequencyStrip typeBuckets={vm.typeBuckets} />
	</section>

	{#if vm.typeBuckets.length > 0}
		<section>
			<div
				class="text-surface-700-200 mb-2 flex items-baseline gap-2 text-xs font-semibold tracking-wide uppercase"
			>
				Filter
				<span class="text-surface-500-400 font-normal normal-case">
					{#if selectedTypes.size === 0}
						showing all
					{:else}
						{selectedTypes.size} selected
					{/if}
				</span>
			</div>
			<TypeFilter typeBuckets={vm.typeBuckets} {selectedTypes} />
		</section>
	{/if}

	<EventFeed
		events={vm.filteredEvents}
		{players}
		{isTeamGame}
		{latestTick}
		totalCount={vm.totalCount}
		filterActive={vm.filterActive}
	/>
</div>
