<script lang="ts">
	// Postgame tab — final summary for the just-ended match. Source of truth
	// is scraperWS.previousGames[name]: when null we render EmptyState; when
	// populated we lay out four collapsible sections (Headline / Scoreboard /
	// Player totals / Event log) using the same SvelteSet + localStorage
	// pattern as overview/OverviewTab.svelte.
	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { Accordion } from '@skeletonlabs/skeleton-svelte';
	import { ChevronDownIcon } from '@lucide/svelte';
	import { scraperWS } from '$lib/stores/scraper-ws.svelte';
	import { useDebugContext } from '../context.js';
	import { buildPostgameVm } from './postgame-vm';
	import WinnerHeadline from './WinnerHeadline.svelte';
	import FinalScoreboardSection from './FinalScoreboardSection.svelte';
	import PlayerTotalsSection from './PlayerTotalsSection.svelte';
	import EventLogSection from './EventLogSection.svelte';
	import EmptyState from './EmptyState.svelte';

	let { name }: { name: string } = $props();

	const ctx = useDebugContext();
	const vm = $derived.by(() => buildPostgameVm(name, scraperWS, ctx));

	// Section IDs — driven by what content is present.
	type SectionId = 'headline' | 'scoreboard' | 'player_totals' | 'event_log';

	let collapsedSections = $state(new SvelteSet<string>());

	function loadCollapsed() {
		try {
			const raw = localStorage.getItem('debug.postgame.collapsed');
			if (raw) collapsedSections = new SvelteSet<string>(JSON.parse(raw));
		} catch {
			// localStorage unavailable; default to all open.
		}
	}

	function saveCollapsed() {
		try {
			localStorage.setItem('debug.postgame.collapsed', JSON.stringify([...collapsedSections]));
		} catch {
			// ignore
		}
	}

	const visibleSections = $derived.by<SectionId[]>(() => {
		const visible: SectionId[] = ['headline'];
		if (vm.finalScoreRows.length > 0 || vm.finalPlayerScoreTiles.length > 0) {
			visible.push('scoreboard');
		}
		if (vm.playerTotalsFlat.length > 0) visible.push('player_totals');
		if (vm.events.length > 0) visible.push('event_log');
		return visible;
	});

	const openSections = $derived(visibleSections.filter((id) => !collapsedSections.has(id)));

	function onAccordionChange(next: string[]) {
		const open = new Set(next);
		const newCollapsed = new SvelteSet<string>();
		for (const id of visibleSections) {
			if (!open.has(id)) newCollapsed.add(id);
		}
		collapsedSections = newCollapsed;
		saveCollapsed();
	}

	onMount(loadCollapsed);
</script>

{#snippet trigger(title: string, subtitle?: string)}
	<span
		class="text-surface-700-200 inline-flex items-baseline gap-2 text-xs font-semibold tracking-wide uppercase"
	>
		{title}
		{#if subtitle}
			<span class="text-surface-500-400 font-normal normal-case">{subtitle}</span>
		{/if}
	</span>
{/snippet}

{#if !vm.previousGame}
	<EmptyState />
{:else}
	<Accordion value={openSections} onValueChange={(e) => onAccordionChange(e.value)} multiple>
		<Accordion.Item value="headline">
			<Accordion.ItemTrigger
				class="group flex w-full items-center justify-between gap-2 py-2 text-left"
			>
				{@render trigger('Result', vm.endedAtStr)}
				<Accordion.ItemIndicator>
					<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
				</Accordion.ItemIndicator>
			</Accordion.ItemTrigger>
			<Accordion.ItemContent class="pb-3">
				<WinnerHeadline winner={vm.winner} gameData={vm.gameData} endedAtStr={vm.endedAtStr} />
			</Accordion.ItemContent>
		</Accordion.Item>

		{#if vm.finalScoreRows.length > 0 || vm.finalPlayerScoreTiles.length > 0}
			<Accordion.Item value="scoreboard">
				<Accordion.ItemTrigger
					class="group flex w-full items-center justify-between gap-2 py-2 text-left"
				>
					{@render trigger('Final scoreboard')}
					<Accordion.ItemIndicator>
						<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
					</Accordion.ItemIndicator>
				</Accordion.ItemTrigger>
				<Accordion.ItemContent class="pb-3">
					<FinalScoreboardSection
						showHeader={false}
						finalScoreRows={vm.finalScoreRows}
						finalPlayerScoreTiles={vm.finalPlayerScoreTiles}
					/>
				</Accordion.ItemContent>
			</Accordion.Item>
		{/if}

		{#if vm.playerTotalsFlat.length > 0}
			<Accordion.Item value="player_totals">
				<Accordion.ItemTrigger
					class="group flex w-full items-center justify-between gap-2 py-2 text-left"
				>
					{@render trigger('Player totals', `(${vm.playerTotalsFlat.length})`)}
					<Accordion.ItemIndicator>
						<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
					</Accordion.ItemIndicator>
				</Accordion.ItemTrigger>
				<Accordion.ItemContent class="pb-3">
					<PlayerTotalsSection
						showHeader={false}
						playerTotalsByTeam={vm.playerTotalsByTeam}
						playerTotalsFlat={vm.playerTotalsFlat}
						isTeamGame={vm.isTeamGame}
					/>
				</Accordion.ItemContent>
			</Accordion.Item>
		{/if}

		{#if vm.events.length > 0}
			<Accordion.Item value="event_log">
				<Accordion.ItemTrigger
					class="group flex w-full items-center justify-between gap-2 py-2 text-left"
				>
					{@render trigger(
						'Event log',
						`${vm.events.length} ${vm.events.length === 1 ? 'event' : 'events'}`
					)}
					<Accordion.ItemIndicator>
						<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
					</Accordion.ItemIndicator>
				</Accordion.ItemTrigger>
				<Accordion.ItemContent class="pb-3">
					<EventLogSection
						showHeader={false}
						events={vm.events}
						players={vm.gameData?.players ?? []}
						isTeamGame={vm.isTeamGame}
					/>
				</Accordion.ItemContent>
			</Accordion.Item>
		{/if}
	</Accordion>
{/if}
