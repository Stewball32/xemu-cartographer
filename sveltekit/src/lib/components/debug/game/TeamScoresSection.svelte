<script lang="ts">
	// Team-scores grid for the Game tab. Renders GameData.team_scores via the
	// shared ScoreTile (label + progress bar against score_limit). Only useful
	// for team games — FFA teams scores live on the player list instead.

	import ScoreTile from '../shared/ScoreTile.svelte';
	import type { GameScoreRow } from './game-vm';

	let {
		scoreRows,
		showHeader = true
	}: {
		scoreRows: GameScoreRow[];
		showHeader?: boolean;
	} = $props();
</script>

<section>
	{#if showHeader}
		<div class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			Team scores
		</div>
	{/if}
	{#if scoreRows.length === 0}
		<div class="text-surface-500-400 card preset-tonal p-3 text-sm">no team scores</div>
	{:else}
		<div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
			{#each scoreRows as row, i (i)}
				<ScoreTile label={row.label} value={row.value} limit={row.limit} accent={row.accent} />
			{/each}
		</div>
	{/if}
</section>
