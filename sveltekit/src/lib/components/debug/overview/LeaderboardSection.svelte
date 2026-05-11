<script lang="ts">
	// Leaderboard tiles — team scores (with progress bars) for team games,
	// or one mini-score-tile per player (sorted by score descending) for FFA.
	// Split out of MatchHeader so OverviewTab can wrap it in its own
	// accordion item independent of the game-context tiles.

	import { TrophyIcon } from '@lucide/svelte';
	import StatTile from '../shared/StatTile.svelte';
	import ScoreTile from './ScoreTile.svelte';
	import type { PlayerScoreTile, ScoreRow } from './overview-vm';

	let {
		scoreRows,
		playerScoreTiles,
		showHeader = true
	}: {
		scoreRows: ScoreRow[];
		playerScoreTiles: PlayerScoreTile[];
		showHeader?: boolean;
	} = $props();

	const hasContent = $derived(scoreRows.length > 0 || playerScoreTiles.length > 0);
</script>

{#if hasContent}
	<section>
		{#if showHeader}
			<div class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
				Leaderboard
			</div>
		{/if}
		{#if scoreRows.length > 0}
			<div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
				{#each scoreRows as row, i (i)}
					<ScoreTile label={row.label} value={row.value} limit={row.limit} accent={row.accent} />
				{/each}
			</div>
		{:else}
			<div class="grid grid-cols-2 gap-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
				{#each playerScoreTiles as p (p.name + ':' + p.rank)}
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
{/if}
