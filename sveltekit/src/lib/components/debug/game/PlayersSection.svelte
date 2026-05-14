<script lang="ts">
	// Players section for the Game tab — the merged home for everything
	// roster-shaped. Replaces the old separate team_scores / player_totals /
	// players sections: per-team accordions (team games) or a flat armor-tinted
	// grid (FFA), each cell a PlayerStatsCard surfacing all 19 GamePlayer wire
	// fields. The team_scores payload rides in the accordion header as
	// score / limit + a thin progress bar (absorbing the old ScoreTile visual).
	//
	// Data comes pre-sorted/grouped from buildPlayerTotals (team games: grouped
	// by team, score-desc within; FFA: flat, score-desc) so this component owns
	// no sorting of its own.

	import { SvelteSet } from 'svelte/reactivity';
	import { Accordion } from '@skeletonlabs/skeleton-svelte';
	import { ChevronDownIcon } from '@lucide/svelte';
	import PlayerStatsCard from '../shared/PlayerStatsCard.svelte';
	import { armorAccent, armorLabel, teamAccent, teamLabel } from '../shared/util';
	import type { PlayerTotalRow } from '../postgame/postgame-vm';
	import type { GameScoreRow } from './game-vm';

	let {
		playerTotalsByTeam,
		playerTotalsFlat,
		scoreRows,
		isTeamGame,
		showHeader = true
	}: {
		playerTotalsByTeam: Array<{ team: number; rows: PlayerTotalRow[] }>;
		playerTotalsFlat: PlayerTotalRow[];
		scoreRows: GameScoreRow[];
		isTeamGame: boolean;
		showHeader?: boolean;
	} = $props();

	function scoreRowFor(team: number): GameScoreRow | undefined {
		return scoreRows.find((s) => s.team === team);
	}

	// Bar fill % — mirrors ScoreTile's clamp. 0 when there's no score limit
	// (the header then reads "no limit" and the bar is hidden).
	function scorePct(s: GameScoreRow | undefined): number {
		if (!s || s.limit <= 0) return 0;
		return Math.max(0, Math.min(100, (s.value / s.limit) * 100));
	}

	let collapsedTeams = $state(new SvelteSet<string>());
	const openTeams = $derived(
		playerTotalsByTeam.map((g) => String(g.team)).filter((v) => !collapsedTeams.has(v))
	);
	function onAccordionChange(next: string[]) {
		const open = new Set(next);
		const newCollapsed = new SvelteSet<string>();
		for (const g of playerTotalsByTeam) {
			const v = String(g.team);
			if (!open.has(v)) newCollapsed.add(v);
		}
		collapsedTeams = newCollapsed;
	}

	function colsClass(count: number): string {
		if (count <= 1) return 'grid-cols-1';
		if (count === 2) return 'grid-cols-1 lg:grid-cols-2';
		return 'grid-cols-1 lg:grid-cols-2 2xl:grid-cols-3';
	}
</script>

<section>
	{#if showHeader}
		<div class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			Players ({playerTotalsFlat.length})
		</div>
	{/if}

	{#if playerTotalsFlat.length === 0}
		<div class="text-surface-500-400 card preset-tonal p-3 text-sm">no players</div>
	{:else if isTeamGame}
		<Accordion value={openTeams} onValueChange={(e) => onAccordionChange(e.value)} multiple>
			{#each playerTotalsByTeam as group (group.team)}
				{@const accent = teamAccent(group.team)}
				{@const score = scoreRowFor(group.team)}
				<Accordion.Item value={String(group.team)}>
					<Accordion.ItemTrigger
						class="group flex w-full items-center justify-between gap-2 rounded px-2 py-1 text-xs font-semibold uppercase {accent.bg}"
					>
						<span class="flex items-center gap-2">
							<span class="block size-3 rounded-sm {accent.dot}"></span>
							{teamLabel(group.team)}
							<span class="text-surface-700-200 font-normal normal-case tabular-nums">
								({group.rows.length})
							</span>
						</span>
						<span class="flex items-center gap-2 normal-case">
							{#if score}
								<span class="font-mono tabular-nums">
									<span class="font-bold">{score.value}</span>
									{#if score.limit > 0}
										<span class="text-surface-700-200">/{score.limit}</span>
									{:else}
										<span class="text-surface-700-200 ml-1 text-[10px] italic">no limit</span>
									{/if}
								</span>
								{#if score.limit > 0}
									<div
										class="relative h-1 w-16 overflow-hidden rounded-full bg-surface-300-700 sm:w-24"
									>
										<div class="h-full {accent.dot}" style="width: {scorePct(score)}%"></div>
									</div>
								{/if}
							{/if}
							<Accordion.ItemIndicator>
								<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
							</Accordion.ItemIndicator>
						</span>
					</Accordion.ItemTrigger>
					<Accordion.ItemContent class="pt-2 pb-3">
						<div class="grid gap-2 {colsClass(group.rows.length)}">
							{#each group.rows as row (row.index)}
								<PlayerStatsCard {row} />
							{/each}
						</div>
					</Accordion.ItemContent>
				</Accordion.Item>
			{/each}
		</Accordion>
	{:else}
		<div class="grid gap-2 {colsClass(playerTotalsFlat.length)}">
			{#each playerTotalsFlat as row (row.index)}
				{@const armor = armorAccent(row.armor_color)}
				<div class="rounded {armor.bg}" title={armorLabel(row.armor_color)}>
					<PlayerStatsCard {row} />
				</div>
			{/each}
		</div>
	{/if}
</section>
