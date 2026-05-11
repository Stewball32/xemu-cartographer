<script lang="ts">
	import { onMount } from 'svelte';
	import { Accordion } from '@skeletonlabs/skeleton-svelte';
	import { ChevronDownIcon } from '@lucide/svelte';
	import { scraperWS } from '$lib/stores/scraper-ws.svelte';
	import { useDebugContext } from '../context.js';
	import { buildOverviewVm } from './overview-vm';
	import ScraperStatusStrip from './ScraperStatusStrip.svelte';
	import GameSection from './GameSection.svelte';
	import LeaderboardSection from './LeaderboardSection.svelte';
	import Roster from './Roster.svelte';
	import StateInputsGrid from './StateInputsGrid.svelte';
	import RecentEvents from './RecentEvents.svelte';
	import PreviousMatch from './PreviousMatch.svelte';

	let { name }: { name: string } = $props();

	const ctx = useDebugContext();
	const vm = $derived.by(() => buildOverviewVm(name, scraperWS, ctx));

	// Section IDs — driven by which content is visible. Keep in priority
	// order (top-of-page first).
	const allSectionIds = [
		'server',
		'game',
		'leaderboard',
		'roster',
		'state_inputs',
		'recent_events',
		'previous_match'
	] as const;
	type SectionId = (typeof allSectionIds)[number];

	// Persisted-in-localStorage collapsed set. Same pattern Roster uses for
	// per-team collapse — the inverse (which are open) feeds the Accordion's
	// value prop, and toggle events update the collapsed set.
	let collapsedSections = $state(new Set<string>());

	function loadCollapsed() {
		try {
			const raw = localStorage.getItem('debug.overview.collapsed');
			if (raw) collapsedSections = new Set<string>(JSON.parse(raw));
		} catch {
			// localStorage unavailable; default to all open.
		}
	}

	function saveCollapsed() {
		try {
			localStorage.setItem('debug.overview.collapsed', JSON.stringify([...collapsedSections]));
		} catch {
			// ignore
		}
	}

	const visibleSections = $derived.by<SectionId[]>(() => {
		const visible: SectionId[] = ['server'];
		if (vm.gameData) visible.push('game');
		if (vm.scoreRows.length > 0 || vm.playerScoreTiles.length > 0) visible.push('leaderboard');
		visible.push('roster');
		visible.push('state_inputs');
		visible.push('recent_events');
		if (vm.previousGame) visible.push('previous_match');
		return visible;
	});

	const openSections = $derived(visibleSections.filter((id) => !collapsedSections.has(id)));

	function onAccordionChange(next: string[]) {
		const open = new Set(next);
		const newCollapsed = new Set<string>();
		for (const id of visibleSections) {
			if (!open.has(id)) newCollapsed.add(id);
		}
		collapsedSections = newCollapsed;
		saveCollapsed();
	}

	onMount(loadCollapsed);

	// Auto-open previous_match when we want it to draw attention (postgame
	// state, or between matches). User-driven collapse still wins on next
	// load via localStorage.
	$effect(() => {
		if (vm.previousMatchAutoOpen) {
			collapsedSections.delete('previous_match');
			collapsedSections = new Set(collapsedSections);
		}
	});
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

<Accordion value={openSections} onValueChange={(e) => onAccordionChange(e.value)} multiple>
	<Accordion.Item value="server">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render trigger('Server')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<ScraperStatusStrip
				showHeader={false}
				phase={vm.phase}
				lastReadStr={vm.lastReadStr}
				gameDataStr={vm.gameDataStr}
				tickStr={vm.tickStr}
				tickValue={vm.tickValue}
				wsConnected={vm.wsConnected}
				lastEventsReplyStr={vm.lastEventsReplyStr}
				lastEventsReplyPhase={vm.lastEventsReplyPhase}
			/>
		</Accordion.ItemContent>
	</Accordion.Item>

	{#if vm.gameData}
		<Accordion.Item value="game">
			<Accordion.ItemTrigger
				class="group flex w-full items-center justify-between gap-2 py-2 text-left"
			>
				{@render trigger('Game')}
				<Accordion.ItemIndicator>
					<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
				</Accordion.ItemIndicator>
			</Accordion.ItemTrigger>
			<Accordion.ItemContent class="pb-3">
				<GameSection showHeader={false} gameData={vm.gameData} tickValue={vm.tickValue} />
			</Accordion.ItemContent>
		</Accordion.Item>
	{/if}

	{#if vm.scoreRows.length > 0 || vm.playerScoreTiles.length > 0}
		<Accordion.Item value="leaderboard">
			<Accordion.ItemTrigger
				class="group flex w-full items-center justify-between gap-2 py-2 text-left"
			>
				{@render trigger('Leaderboard')}
				<Accordion.ItemIndicator>
					<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
				</Accordion.ItemIndicator>
			</Accordion.ItemTrigger>
			<Accordion.ItemContent class="pb-3">
				<LeaderboardSection
					showHeader={false}
					scoreRows={vm.scoreRows}
					playerScoreTiles={vm.playerScoreTiles}
				/>
			</Accordion.ItemContent>
		</Accordion.Item>
	{/if}

	<Accordion.Item value="roster">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render trigger('Roster', vm.gameData ? `(${vm.gameData.players?.length ?? 0})` : undefined)}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			{#if vm.gameData && (vm.gameData.players ?? []).length > 0}
				<Roster showHeader={false} gameData={vm.gameData} tick={vm.tick} />
			{:else}
				<div class="text-surface-500-400 card preset-tonal p-3 text-sm">
					{vm.gameData ? 'no players in current match' : 'no game data — waiting for first read'}
				</div>
			{/if}
		</Accordion.ItemContent>
	</Accordion.Item>

	<Accordion.Item value="state_inputs">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render trigger('State inputs', 'diagnostic')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<StateInputsGrid showHeader={false} stateInputs={vm.stateInputs} />
		</Accordion.ItemContent>
	</Accordion.Item>

	<Accordion.Item value="recent_events">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render trigger(
				'Recent events',
				vm.events.length === 0
					? 'none yet'
					: `${Math.min(10, vm.events.length)} of ${vm.events.length}`
			)}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<RecentEvents
				showHeader={false}
				events={vm.events}
				players={vm.gameData?.players ?? []}
				isTeamGame={vm.gameData?.is_team_game === true}
				latestTick={vm.tickValue}
			/>
		</Accordion.ItemContent>
	</Accordion.Item>

	{#if vm.previousGame}
		<Accordion.Item value="previous_match">
			<Accordion.ItemTrigger
				class="group flex w-full items-center justify-between gap-2 py-2 text-left"
			>
				{@render trigger('Previous match')}
				<Accordion.ItemIndicator>
					<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
				</Accordion.ItemIndicator>
			</Accordion.ItemTrigger>
			<Accordion.ItemContent class="pb-3">
				<PreviousMatch previousGame={vm.previousGame} endedAtMs={vm.previousGameEndedAtMs} />
			</Accordion.ItemContent>
		</Accordion.Item>
	{/if}
</Accordion>
