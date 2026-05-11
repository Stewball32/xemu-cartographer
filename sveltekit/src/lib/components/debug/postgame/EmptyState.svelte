<script lang="ts">
	// Postgame-tab empty state. Shown when scraperWS.previousGames[name] is
	// null — the runner is still on its first match, or the previous-match
	// snapshot has been dropped (Ready → Idle).
	import { TrophyIcon } from '@lucide/svelte';
</script>

<div class="mx-auto flex max-w-md flex-col items-center gap-3 card preset-tonal p-6 text-center">
	<span
		class="text-surface-500-400 flex size-12 items-center justify-center rounded-full bg-surface-200-800"
	>
		<TrophyIcon class="size-6" />
	</span>
	<div>
		<div class="text-base font-semibold">No previous match recorded yet.</div>
		<p class="text-surface-700-200 mt-1 text-sm">
			Finish a match and the postgame summary will appear here — winning team, final scoreboard,
			per-player totals, and the full event log.
		</p>
	</div>
</div>
