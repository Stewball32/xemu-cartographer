<script lang="ts">
	import type { TickPlayer, GamePlayer } from '$lib/types/scraper';

	let {
		tickPlayer,
		gamePlayer,
		selected = false,
		teamGame = false,
		onSelect
	}: {
		tickPlayer: TickPlayer;
		gamePlayer: GamePlayer | null;
		selected?: boolean;
		teamGame?: boolean;
		onSelect: () => void;
	} = $props();

	const name = $derived(gamePlayer?.name ?? `Player ${tickPlayer.index}`);
	const teamColor = $derived.by(() => {
		if (!teamGame) return 'bg-surface-500';
		const t = gamePlayer?.team ?? 0;
		return t === 0 ? 'bg-error-500' : 'bg-primary-500';
	});

	// Halo CE caps health at 1.0, shields at 1.0 (normalized).
	const hpPct = $derived(Math.max(0, Math.min(100, Math.round((tickPlayer.health ?? 0) * 100))));
	const shieldPct = $derived(Math.max(0, Math.min(100, Math.round((tickPlayer.shields ?? 0) * 100))));
</script>

<button
	type="button"
	onclick={onSelect}
	class="w-full rounded p-2 text-left transition {selected
		? 'bg-primary-500/20 ring-primary-500 ring-1'
		: 'hover:bg-surface-200-800'}"
>
	<div class="flex items-center gap-2">
		<span
			class="size-2 rounded-full {tickPlayer.alive ? 'bg-success-500' : 'bg-error-500'}"
			title={tickPlayer.alive ? 'alive' : 'dead'}
		></span>
		{#if teamGame}
			<span class="size-3 rounded-sm {teamColor}" title="team {gamePlayer?.team ?? 0}"></span>
		{/if}
		<span class="flex-1 truncate text-sm font-medium">{name}</span>
		<span class="text-surface-700-200 font-mono text-xs">#{tickPlayer.index}</span>
	</div>
	<div class="mt-1.5 space-y-0.5">
		<div class="flex items-center gap-1.5">
			<span class="text-surface-700-200 w-6 text-[10px] font-mono">HP</span>
			<div class="bg-surface-300-700 h-1.5 flex-1 overflow-hidden rounded-full">
				<div class="bg-success-500 h-full" style="width: {hpPct}%"></div>
			</div>
			<span class="font-mono text-[10px] tabular-nums w-8 text-right">{hpPct}%</span>
		</div>
		<div class="flex items-center gap-1.5">
			<span class="text-surface-700-200 w-6 text-[10px] font-mono">Sh</span>
			<div class="bg-surface-300-700 h-1.5 flex-1 overflow-hidden rounded-full">
				<div class="bg-tertiary-500 h-full" style="width: {shieldPct}%"></div>
			</div>
			<span class="font-mono text-[10px] tabular-nums w-8 text-right">{shieldPct}%</span>
		</div>
	</div>
	{#if gamePlayer}
		<div class="text-surface-700-200 mt-1.5 flex gap-3 font-mono text-[10px]">
			<span>K {gamePlayer.kills}</span>
			<span>D {gamePlayer.deaths}</span>
			<span>A {gamePlayer.assists}</span>
		</div>
	{/if}
</button>
