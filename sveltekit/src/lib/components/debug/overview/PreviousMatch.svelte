<script lang="ts">
	import type { PreviousGameInfo } from '$lib/types/scraper';
	import { useDebugContext } from '../context';

	let {
		previousGame,
		endedAtMs
	}: {
		previousGame: PreviousGameInfo;
		endedAtMs: number | undefined;
	} = $props();

	const ctx = useDebugContext();
</script>

<div class="space-y-1.5">
	<div class="text-surface-700-200 text-xs">ended {ctx.relativeTime(endedAtMs)}</div>
	{#if previousGame.game_data}
		<div class="text-sm">
			<span class="font-mono">{previousGame.game_data.map || '—'}</span>
			·
			<span class="font-mono">{previousGame.game_data.gametype}</span>
			·
			<span class="font-mono tabular-nums">
				{previousGame.game_data.players?.length ?? 0} players
			</span>
			{#if previousGame.events && previousGame.events.length > 0}
				·
				<span class="font-mono tabular-nums">
					{previousGame.events.length} events
				</span>
			{/if}
		</div>
	{:else}
		<div class="text-surface-500-400 text-sm">(no game data captured for the previous match)</div>
	{/if}
</div>
