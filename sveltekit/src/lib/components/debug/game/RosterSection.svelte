<script lang="ts">
	// Roster section for the Game tab — same grouping logic as
	// overview/Roster.svelte but without tick data (static per-match config: HP
	// and shields don't belong here). PlayerCard already renders gracefully when
	// `t` is undefined: alive-dot becomes 'unknown', meters read 0/—.

	import { SvelteMap, SvelteSet } from 'svelte/reactivity';
	import { Accordion } from '@skeletonlabs/skeleton-svelte';
	import { ChevronDownIcon } from '@lucide/svelte';
	import type { GameData, GamePlayer } from '$lib/types/scraper';
	import PlayerCard from '../shared/PlayerCard.svelte';
	import { armorAccent, armorLabel, teamAccent, teamLabel } from '../shared/util';

	let {
		gameData,
		showHeader = true
	}: {
		gameData: GameData;
		showHeader?: boolean;
	} = $props();

	const isTeam = $derived(gameData.is_team_game === true);
	const players = $derived<GamePlayer[]>(gameData.players ?? []);
	const teamScores = $derived(gameData.team_scores ?? []);

	const machineNameByIndex = $derived.by(() => {
		const map = new SvelteMap<number, string>();
		for (const m of gameData.machines ?? []) map.set(m.index, m.name);
		return map;
	});

	function machineLabel(p: GamePlayer): string | null {
		if (p.machine_index === null || p.machine_index === undefined) return null;
		return machineNameByIndex.get(p.machine_index) ?? `M${p.machine_index}`;
	}

	const playersByTeam = $derived.by(() => {
		const map = new SvelteMap<number, GamePlayer[]>();
		for (const p of players) {
			const arr = map.get(p.team) ?? [];
			arr.push(p);
			map.set(p.team, arr);
		}
		for (const arr of map.values()) {
			arr.sort((a, b) => (a.name || '').localeCompare(b.name || ''));
		}
		return [...map.entries()].sort(([a], [b]) => a - b);
	});

	const sortedPlayers = $derived(
		[...players].sort((a, b) => {
			if (a.team !== b.team) return a.team - b.team;
			return (a.name || '').localeCompare(b.name || '');
		})
	);

	function teamScoreFor(team: number): number {
		return teamScores.find((t) => t.team === team)?.score ?? 0;
	}

	let collapsedTeams = $state(new SvelteSet<string>());
	const openTeams = $derived(
		playersByTeam.map(([t]) => String(t)).filter((v) => !collapsedTeams.has(v))
	);
	function onAccordionChange(next: string[]) {
		const open = new Set(next);
		const newCollapsed = new SvelteSet<string>();
		for (const [t] of playersByTeam) {
			const v = String(t);
			if (!open.has(v)) newCollapsed.add(v);
		}
		collapsedTeams = newCollapsed;
	}

	function colsClass(count: number): string {
		if (count <= 1) return 'grid-cols-1';
		if (count === 2) return 'grid-cols-1 sm:grid-cols-2';
		if (count === 3) return 'grid-cols-1 sm:grid-cols-2 md:grid-cols-3';
		return 'grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4';
	}
</script>

<section>
	{#if showHeader}
		<div class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			Roster ({players.length})
		</div>
	{/if}

	{#if players.length === 0}
		<div class="text-surface-500-400 card preset-tonal p-3 text-sm">no players</div>
	{:else if isTeam}
		<Accordion value={openTeams} onValueChange={(e) => onAccordionChange(e.value)} multiple>
			{#each playersByTeam as [team, members] (team)}
				{@const accent = teamAccent(team)}
				<Accordion.Item value={String(team)}>
					<Accordion.ItemTrigger
						class="group flex w-full items-center justify-between gap-2 rounded px-2 py-1 text-xs font-semibold uppercase {accent.bg}"
					>
						<span class="flex items-center gap-2">
							<span class="block size-3 rounded-sm {accent.dot}"></span>
							{teamLabel(team)}
							<span class="text-surface-700-200 font-normal normal-case tabular-nums">
								({members.length})
							</span>
						</span>
						<span class="flex items-center gap-3 normal-case">
							<span class="font-mono tabular-nums">{teamScoreFor(team)} pts</span>
							<Accordion.ItemIndicator>
								<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
							</Accordion.ItemIndicator>
						</span>
					</Accordion.ItemTrigger>
					<Accordion.ItemContent class="pt-2 pb-3">
						<div class="grid gap-2 {colsClass(members.length)}">
							{#each members as p (p.index)}
								<PlayerCard {p} t={undefined} {machineLabel} {isTeam} />
							{/each}
						</div>
					</Accordion.ItemContent>
				</Accordion.Item>
			{/each}
		</Accordion>
	{:else}
		<div class="grid gap-2 {colsClass(sortedPlayers.length)}">
			{#each sortedPlayers as p (p.index)}
				{@const armor = armorAccent(p.armor_color)}
				<div class="rounded {armor.bg}" title={armorLabel(p.armor_color)}>
					<PlayerCard {p} t={undefined} {machineLabel} {isTeam} />
				</div>
			{/each}
		</div>
	{/if}
</section>
