<script lang="ts">
	// In-app live / final scoreboard for the play tab. Reuses the same pure
	// view-builders as the OBS overlays (buildScoreboard / matchClock /
	// statusStrip) so the numbers match the broadcast graphics, but renders with
	// Skeleton surface tokens instead of the transparent broadcast canvas.
	import { buildScoreboard, matchClock, statusStrip } from '$lib/utils/overlay-view';
	import type { GamePayload, ScenarioPayload, TickPayloadV2 } from '$lib/types/scraper-v2';
	import { CircleDotIcon } from '@lucide/svelte';

	interface Props {
		game: GamePayload | null;
		tick: TickPayloadV2 | null;
		scenario: ScenarioPayload | null;
		/** Post-game presentation: "FINAL" treatment + a back-to-lobby button. */
		final?: boolean;
		onback?: () => void;
	}

	let { game, tick, scenario, final = false, onback }: Props = $props();

	const vm = $derived(buildScoreboard(game, tick));
	const clock = $derived(matchClock(game));
	const status = $derived(statusStrip(game, scenario));
</script>

<div class="flex flex-col gap-3">
	<!-- Match header -->
	<div class="flex flex-wrap items-center gap-x-4 gap-y-1 card preset-tonal p-3">
		<div class="flex items-center gap-2">
			{#if final}
				<span class="badge preset-filled-warning-500">FINAL</span>
			{:else if clock.live}
				<span class="badge flex items-center gap-1 preset-filled-error-500">
					<CircleDotIcon class="size-3 animate-pulse" /> LIVE
				</span>
			{/if}
			<span class="font-bold tracking-wide uppercase">{vm.gametype || 'Match'}</span>
		</div>
		{#if status.map}
			<span class="text-sm text-surface-600-400">{status.map}</span>
		{/if}
		<span class="ml-auto font-mono text-2xl font-bold tabular-nums">{clock.label}</span>
		<span class="text-[0.6rem] font-semibold tracking-widest text-surface-500 uppercase">
			{clock.direction === 'down' ? 'Remaining' : 'Elapsed'}
		</span>
		{#if status.scoreLimit > 0}
			<span class="text-sm text-surface-600-400">to {status.scoreLimit}</span>
		{/if}
	</div>

	{#if vm.playerCount === 0}
		<div class="card preset-tonal p-6 text-center text-sm text-surface-600-400">
			Waiting for players to join…
		</div>
	{:else if vm.isTeamGame}
		<div class="grid gap-3 sm:grid-cols-2">
			{#each vm.teams as t (t.team)}
				<section class="overflow-hidden card p-0" style="border-top: 3px solid {t.color}">
					<header
						class="flex items-center gap-2 px-3 py-2"
						style="background: color-mix(in srgb, {t.color} 22%, transparent)"
					>
						<span class="font-bold tracking-wide uppercase" style="color: {t.color}">{t.name}</span>
						{#if vm.hasScores}
							<span class="ml-auto font-mono text-xl font-bold tabular-nums">{t.score}</span>
						{/if}
					</header>
					<div class="flex flex-col divide-y divide-surface-200-800">
						{#each t.players as p (p.index)}
							{@render row(p, t.color)}
						{/each}
					</div>
				</section>
			{/each}
		</div>
	{:else}
		<section class="overflow-hidden card p-0">
			<div class="flex flex-col divide-y divide-surface-200-800">
				{#each vm.players as p, i (p.index)}
					{@render row(p, '#8a94a6', i + 1)}
				{/each}
			</div>
		</section>
	{/if}

	{#if !vm.hasScores && vm.playerCount > 0}
		<p class="text-center text-xs text-surface-500">Scores populate once the match is underway.</p>
	{/if}

	{#if final && onback}
		<div class="flex justify-center pt-1">
			<button type="button" class="btn preset-filled-primary-500" onclick={onback}>
				Back to lobby
			</button>
		</div>
	{/if}
</div>

{#snippet row(
	p: ReturnType<typeof buildScoreboard>['players'][number],
	color: string,
	rank?: number
)}
	<div class="flex items-center gap-2 px-3 py-1.5" class:opacity-40={vm.hasTick && !p.alive}>
		{#if rank}
			<span class="w-4 text-center font-mono text-xs text-surface-500 tabular-nums">{rank}</span>
		{/if}
		<span
			class="size-2.5 shrink-0 rounded-sm"
			style="background: {color}; box-shadow: 0 0 6px {color}"
		></span>
		<span class="min-w-0 flex-1 truncate text-sm font-medium" title={p.name}>{p.name}</span>
		{#if p.hasOvershield}<span class="preset-tonal-warning-500 badge text-[0.55rem]">OS</span>{/if}
		{#if p.hasCamo}<span class="preset-tonal-primary-500 badge text-[0.55rem]">CAMO</span>{/if}
		<span
			class="font-mono text-xs text-surface-600-400 tabular-nums"
			class:opacity-40={!vm.hasScores}
		>
			{vm.hasScores ? `${p.kills}/${p.deaths}/${p.assists}` : '—'}
		</span>
		<span class="w-8 text-right font-mono text-sm font-bold tabular-nums">
			{vm.hasScores ? p.score : '—'}
		</span>
	</div>
{/snippet}
