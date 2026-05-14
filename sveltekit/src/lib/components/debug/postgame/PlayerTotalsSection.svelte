<script lang="ts">
	// Per-player end-of-match stats, rendered as a grid of PlayerStatsCard
	// (the shared card also used by the Game tab's merged players section).
	// Team games: one card grid per team under a colored header bar. FFA: a
	// single grid, each card wrapped in an armor-tinted div. Every card
	// surfaces all 19 GamePlayer wire fields grouped into FieldTile clusters.
	import PlayerStatsCard from '../shared/PlayerStatsCard.svelte';
	import { armorAccent, armorLabel, teamAccent, teamLabel } from '../shared/util';
	import type { PlayerTotalRow } from './postgame-vm';

	let {
		playerTotalsByTeam,
		playerTotalsFlat,
		isTeamGame,
		showHeader = true
	}: {
		playerTotalsByTeam: Array<{ team: number; rows: PlayerTotalRow[] }>;
		playerTotalsFlat: PlayerTotalRow[];
		isTeamGame: boolean;
		showHeader?: boolean;
	} = $props();

	const hasContent = $derived(
		isTeamGame ? playerTotalsByTeam.length > 0 : playerTotalsFlat.length > 0
	);
</script>

<section>
	{#if showHeader}
		<div class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			Player totals
		</div>
	{/if}
	{#if !hasContent}
		<div class="text-surface-500-400 card preset-tonal p-3 text-sm">
			no player totals captured for the previous match
		</div>
	{:else if isTeamGame}
		<div class="space-y-3">
			{#each playerTotalsByTeam as group (group.team)}
				{@const accent = teamAccent(group.team)}
				<div>
					<div
						class="flex items-center gap-2 rounded-t px-2 py-1 text-xs font-semibold uppercase {accent.bg}"
					>
						<span class="block size-3 rounded-sm {accent.dot}"></span>
						<span>{teamLabel(group.team)}</span>
						<span class="text-surface-700-200 ml-auto font-normal normal-case tabular-nums">
							{group.rows.length} player{group.rows.length === 1 ? '' : 's'}
						</span>
					</div>
					<div class="mt-2 grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
						{#each group.rows as row (row.index)}
							<PlayerStatsCard {row} />
						{/each}
					</div>
				</div>
			{/each}
		</div>
	{:else}
		<div class="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
			{#each playerTotalsFlat as row (row.index)}
				{@const armor = armorAccent(row.armor_color)}
				<div class="rounded {armor.bg}" title={armorLabel(row.armor_color)}>
					<PlayerStatsCard {row} />
				</div>
			{/each}
		</div>
	{/if}
</section>
