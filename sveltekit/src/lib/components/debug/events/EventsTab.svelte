<script lang="ts">
	// Events tab — envelope-stats header (with Pretty/JSON Switch), then the
	// existing rolling-feed UI (Pretty) or the TreeView+CodeBlock JSON view.
	// The Pretty body keeps its existing interactive type-chip filter; the
	// JSON body walks the rolling log envelope-by-envelope.

	import { SvelteSet } from 'svelte/reactivity';
	import { scraperWSV2 } from '$lib/stores/scraper-ws-v2.svelte';
	import { useDebugContext } from '../context.js';
	import { buildEventsTabVm } from './events-tab-vm';
	import EventsStatsHeader from './EventsStatsHeader.svelte';
	import EventsJson from './EventsJson.svelte';
	import TypeFilter from './TypeFilter.svelte';
	import EventFeed from './EventFeed.svelte';

	let { name }: { name: string } = $props();

	// viewMode lives on the page (debug context) so the choice persists
	// across tabs that opt into the Pretty/JSON toggle. ctx fields are
	// optional so non-tabbed debug pages (probe) can skip wiring them up.
	const ctx = useDebugContext();
	const viewMode = $derived(ctx.viewMode ?? 'pretty');
	const setViewMode = ctx.setViewMode ?? (() => {});

	// SvelteSet (not Set) so the parent's $derived sees membership changes
	// without identity swaps.
	let selectedTypes = $state(new SvelteSet<string>());

	const vm = $derived.by(() => buildEventsTabVm(name, scraperWSV2, selectedTypes));

	// Rolling event log direct from the v2 store — the JSON view + header
	// read this; the Pretty feed reuses the v1-shaped envelopes from the vm.
	const events = $derived(scraperWSV2.events[name] ?? []);

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
	<EventsStatsHeader {events} instance={name} {viewMode} onViewModeChange={setViewMode} />

	{#if viewMode === 'pretty'}
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
	{:else}
		<EventsJson {events} instance={name} />
	{/if}
</div>
