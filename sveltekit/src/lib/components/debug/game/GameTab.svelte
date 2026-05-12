<script lang="ts">
	// Game tab — static per-match configuration. Every field of GameData has a
	// home somewhere on this tab: match config (map / gametype / variant /
	// limits / difficulty), team scores, roster, player spawns, fog, power-item
	// spawns, machines. Read once per match, unchanged until postgame.
	//
	// Section open/close state mirrors overview/OverviewTab — a SvelteSet of
	// collapsed IDs persisted to localStorage under 'debug.game.collapsed'.

	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { Accordion } from '@skeletonlabs/skeleton-svelte';
	import { ChevronDownIcon } from '@lucide/svelte';
	import { scraperWS } from '$lib/stores/scraper-ws.svelte';
	import { useDebugContext } from '../context.js';
	import { buildGameVm } from './game-vm';
	import MatchConfigSection from './MatchConfigSection.svelte';
	import TeamScoresSection from './TeamScoresSection.svelte';
	import RosterSection from './RosterSection.svelte';
	import SpawnsSection from './SpawnsSection.svelte';
	import FogSection from './FogSection.svelte';
	import PowerItemsSection from './PowerItemsSection.svelte';
	import MachinesSection from './MachinesSection.svelte';
	import ObjectTypesSection from './ObjectTypesSection.svelte';
	import TagCacheSection from './TagCacheSection.svelte';
	import PlayerTotalsSection from '../postgame/PlayerTotalsSection.svelte';

	let { name }: { name: string } = $props();

	const ctx = useDebugContext();
	const vm = $derived.by(() => buildGameVm(name, scraperWS, ctx));

	type SectionId =
		| 'match_config'
		| 'team_scores'
		| 'player_totals'
		| 'roster'
		| 'player_spawns'
		| 'fog'
		| 'power_items'
		| 'machines'
		| 'object_types'
		| 'tag_cache';

	let collapsedSections = $state(new SvelteSet<string>());

	function loadCollapsed() {
		try {
			const raw = localStorage.getItem('debug.game.collapsed');
			if (raw) collapsedSections = new SvelteSet<string>(JSON.parse(raw));
		} catch {
			// localStorage unavailable; default to all open.
		}
	}

	function saveCollapsed() {
		try {
			localStorage.setItem('debug.game.collapsed', JSON.stringify([...collapsedSections]));
		} catch {
			// ignore
		}
	}

	const visibleSections = $derived.by<SectionId[]>(() => {
		if (!vm.gameData) return [];
		const visible: SectionId[] = ['match_config'];
		if (vm.scoreRows.length > 0) visible.push('team_scores');
		if (vm.players.length > 0) visible.push('player_totals');
		visible.push('roster');
		if (vm.playerSpawns.length > 0) visible.push('player_spawns');
		if (vm.fog) visible.push('fog');
		if (vm.powerItemSpawns.length > 0) visible.push('power_items');
		if (vm.machines.length > 0) visible.push('machines');
		if (vm.objectTypes.length > 0) visible.push('object_types');
		if (vm.tagCache) visible.push('tag_cache');
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

{#if !vm.gameData}
	<div class="text-surface-500-400 card preset-tonal p-3 text-sm">
		no game data — waiting for first read
	</div>
{:else}
	<Accordion value={openSections} onValueChange={(e) => onAccordionChange(e.value)} multiple>
		<Accordion.Item value="match_config">
			<Accordion.ItemTrigger
				class="group flex w-full items-center justify-between gap-2 py-2 text-left"
			>
				{@render trigger('Match config')}
				<Accordion.ItemIndicator>
					<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
				</Accordion.ItemIndicator>
			</Accordion.ItemTrigger>
			<Accordion.ItemContent class="pb-3">
				<MatchConfigSection showHeader={false} gameData={vm.gameData} />
			</Accordion.ItemContent>
		</Accordion.Item>

		{#if vm.scoreRows.length > 0}
			<Accordion.Item value="team_scores">
				<Accordion.ItemTrigger
					class="group flex w-full items-center justify-between gap-2 py-2 text-left"
				>
					{@render trigger('Team scores')}
					<Accordion.ItemIndicator>
						<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
					</Accordion.ItemIndicator>
				</Accordion.ItemTrigger>
				<Accordion.ItemContent class="pb-3">
					<TeamScoresSection showHeader={false} scoreRows={vm.scoreRows} />
				</Accordion.ItemContent>
			</Accordion.Item>
		{/if}

		{#if vm.players.length > 0}
			<Accordion.Item value="player_totals">
				<Accordion.ItemTrigger
					class="group flex w-full items-center justify-between gap-2 py-2 text-left"
				>
					{@render trigger('Player totals', `(${vm.players.length})`)}
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

		<Accordion.Item value="roster">
			<Accordion.ItemTrigger
				class="group flex w-full items-center justify-between gap-2 py-2 text-left"
			>
				{@render trigger('Roster', `(${vm.players.length})`)}
				<Accordion.ItemIndicator>
					<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
				</Accordion.ItemIndicator>
			</Accordion.ItemTrigger>
			<Accordion.ItemContent class="pb-3">
				<RosterSection showHeader={false} gameData={vm.gameData} />
			</Accordion.ItemContent>
		</Accordion.Item>

		{#if vm.playerSpawns.length > 0}
			<Accordion.Item value="player_spawns">
				<Accordion.ItemTrigger
					class="group flex w-full items-center justify-between gap-2 py-2 text-left"
				>
					{@render trigger('Player spawns', `(${vm.playerSpawns.length})`)}
					<Accordion.ItemIndicator>
						<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
					</Accordion.ItemIndicator>
				</Accordion.ItemTrigger>
				<Accordion.ItemContent class="pb-3">
					<SpawnsSection showHeader={false} spawns={vm.playerSpawns} />
				</Accordion.ItemContent>
			</Accordion.Item>
		{/if}

		{#if vm.fog}
			<Accordion.Item value="fog">
				<Accordion.ItemTrigger
					class="group flex w-full items-center justify-between gap-2 py-2 text-left"
				>
					{@render trigger('Fog')}
					<Accordion.ItemIndicator>
						<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
					</Accordion.ItemIndicator>
				</Accordion.ItemTrigger>
				<Accordion.ItemContent class="pb-3">
					<FogSection showHeader={false} fog={vm.fog} />
				</Accordion.ItemContent>
			</Accordion.Item>
		{/if}

		{#if vm.powerItemSpawns.length > 0}
			<Accordion.Item value="power_items">
				<Accordion.ItemTrigger
					class="group flex w-full items-center justify-between gap-2 py-2 text-left"
				>
					{@render trigger('Power items', `(${vm.powerItemSpawns.length})`)}
					<Accordion.ItemIndicator>
						<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
					</Accordion.ItemIndicator>
				</Accordion.ItemTrigger>
				<Accordion.ItemContent class="pb-3">
					<PowerItemsSection showHeader={false} spawns={vm.powerItemSpawns} />
				</Accordion.ItemContent>
			</Accordion.Item>
		{/if}

		{#if vm.machines.length > 0}
			<Accordion.Item value="machines">
				<Accordion.ItemTrigger
					class="group flex w-full items-center justify-between gap-2 py-2 text-left"
				>
					{@render trigger('Machines', `(${vm.machines.length})`)}
					<Accordion.ItemIndicator>
						<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
					</Accordion.ItemIndicator>
				</Accordion.ItemTrigger>
				<Accordion.ItemContent class="pb-3">
					<MachinesSection showHeader={false} machines={vm.machines} />
				</Accordion.ItemContent>
			</Accordion.Item>
		{/if}

		{#if vm.objectTypes.length > 0}
			<Accordion.Item value="object_types">
				<Accordion.ItemTrigger
					class="group flex w-full items-center justify-between gap-2 py-2 text-left"
				>
					{@render trigger('Object types', `(${vm.objectTypes.length})`)}
					<Accordion.ItemIndicator>
						<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
					</Accordion.ItemIndicator>
				</Accordion.ItemTrigger>
				<Accordion.ItemContent class="pb-3">
					<ObjectTypesSection showHeader={false} objectTypes={vm.objectTypes} />
				</Accordion.ItemContent>
			</Accordion.Item>
		{/if}

		{#if vm.tagCache}
			<Accordion.Item value="tag_cache">
				<Accordion.ItemTrigger
					class="group flex w-full items-center justify-between gap-2 py-2 text-left"
				>
					{@render trigger('Tag cache')}
					<Accordion.ItemIndicator>
						<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
					</Accordion.ItemIndicator>
				</Accordion.ItemTrigger>
				<Accordion.ItemContent class="pb-3">
					<TagCacheSection showHeader={false} tagCache={vm.tagCache} />
				</Accordion.ItemContent>
			</Accordion.Item>
		{/if}
	</Accordion>
{/if}
