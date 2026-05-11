<script lang="ts">
	import { SvelteMap } from 'svelte/reactivity';
	import { Accordion } from '@skeletonlabs/skeleton-svelte';
	import { ChevronDownIcon } from '@lucide/svelte';
	import type { GameData, GamePlayer, TickPayload, TickPlayer } from '$lib/types/scraper';
	import { teamLabel, teamAccent, armorAccent, armorLabel } from '../shared/util';
	import PlayerCard from './PlayerCard.svelte';

	let {
		gameData,
		tick,
		showHeader = true
	}: {
		gameData: GameData;
		tick: TickPayload | null;
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

	const tickByIdx = $derived.by(() => {
		const map = new SvelteMap<number, TickPlayer>();
		for (const p of tick?.players ?? []) map.set(p.index, p);
		return map;
	});

	const playersByTeam = $derived.by(() => {
		const map = new SvelteMap<number, GamePlayer[]>();
		for (const p of players) {
			const arr = map.get(p.team) ?? [];
			arr.push(p);
			map.set(p.team, arr);
		}
		// Within a team, sort alphabetically by gamertag.
		for (const arr of map.values()) {
			arr.sort((a, b) => (a.name || '').localeCompare(b.name || ''));
		}
		return [...map.entries()].sort(([a], [b]) => a - b);
	});

	// FFA flat list: sort by team index, then alphabetically by gamertag.
	const sortedPlayers = $derived(
		[...players].sort((a, b) => {
			if (a.team !== b.team) return a.team - b.team;
			return (a.name || '').localeCompare(b.name || '');
		})
	);

	function teamScoreFor(team: number): number {
		return teamScores.find((t) => t.team === team)?.score ?? 0;
	}

	// Accordion open-state for team-mode roster — bindable so the user can
	// collapse a team. Auto-includes any team we haven't seen yet (so a new
	// team appearing mid-match defaults to expanded rather than hidden).
	let collapsedTeams = $state(new Set<string>());
	const openTeams = $derived(
		playersByTeam.map(([t]) => String(t)).filter((v) => !collapsedTeams.has(v))
	);
	function onAccordionChange(next: string[]) {
		const open = new Set(next);
		const newCollapsed = new Set<string>();
		for (const [t] of playersByTeam) {
			const v = String(t);
			if (!open.has(v)) newCollapsed.add(v);
		}
		collapsedTeams = newCollapsed;
	}

	// Mobile is always 1 column for legibility; cap at 4 so a 16-player FFA
	// doesn't shrink each card past readability. Steps through 2/3/4 only as
	// the count justifies it — a 2-player roster shouldn't stretch into 4
	// half-empty columns. Tailwind needs literal class strings, so the
	// branches return full strings rather than building them dynamically.
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
								<PlayerCard {p} t={tickByIdx.get(p.index)} {machineLabel} {isTeam} />
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
					<PlayerCard {p} t={tickByIdx.get(p.index)} {machineLabel} {isTeam} />
				</div>
			{/each}
		</div>
	{/if}
</section>
