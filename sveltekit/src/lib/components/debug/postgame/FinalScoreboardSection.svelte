<script lang="ts">
	// Final scoreboard for the just-ended match. Mirrors
	// overview/LeaderboardSection.svelte — team scores get ScoreTile rows
	// (highest first); FFA gets a per-player mini-tile grid (ranked).
	// showHeader=false lets the parent Accordion own the section heading.

	import { TrophyIcon } from '@lucide/svelte';
	import ScoreTile from '../shared/ScoreTile.svelte';
	import StatTile from '../shared/StatTile.svelte';
	import type { FinalPlayerScoreTile, FinalScoreRow } from './postgame-vm';

	let {
		finalScoreRows,
		finalPlayerScoreTiles,
		showHeader = true
	}: {
		finalScoreRows: FinalScoreRow[];
		finalPlayerScoreTiles: FinalPlayerScoreTile[];
		showHeader?: boolean;
	} = $props();

	const hasContent = $derived(finalScoreRows.length > 0 || finalPlayerScoreTiles.length > 0);
</script>

{#if hasContent}
	<section>
		{#if showHeader}
			<div class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
				Final scoreboard
			</div>
		{/if}
		{#if finalScoreRows.length > 0}
			<div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
				{#each finalScoreRows as row, i (row.team + ':' + i)}
					<ScoreTile label={row.label} value={row.value} limit={row.limit} accent={row.accent} />
				{/each}
			</div>
		{:else}
			<div class="grid grid-cols-2 gap-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
				{#each finalPlayerScoreTiles as p (p.name + ':' + p.rank)}
					{#snippet rankIcon()}
						{#if p.rank === 1}
							<TrophyIcon class="size-3.5" />
						{:else}
							<span class="font-mono text-[11px] font-bold tabular-nums">#{p.rank}</span>
						{/if}
					{/snippet}
					<StatTile
						label={p.name}
						display={String(p.score)}
						statusKind={p.rank === 1 ? 'on' : 'neutral'}
						icon={rankIcon}
						subtext="K{p.kills} / D{p.deaths} / A{p.assists}"
						title="{p.name} — rank {p.rank}"
					/>
				{/each}
			</div>
		{/if}
	</section>
{:else if showHeader}
	<section>
		<div class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			Final scoreboard
		</div>
		<div class="text-surface-500-400 card preset-tonal p-3 text-sm">
			no scores captured for the previous match
		</div>
	</section>
{:else}
	<div class="text-surface-500-400 card preset-tonal p-3 text-sm">
		no scores captured for the previous match
	</div>
{/if}
