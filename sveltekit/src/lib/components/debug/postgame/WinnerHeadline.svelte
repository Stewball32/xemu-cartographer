<script lang="ts">
	// Hero card for the postgame tab — large winning-team / winner banner plus
	// a multi-row body summarising the gametype, map, and ended-at timestamp.
	// Uses shared/StatTile with the rows slot so the layout stays consistent
	// with the rest of the debug page.
	//
	// In team games the dot/text colour is sourced from teamAccent(); in FFA
	// we fall back to "on" (success). Ties downgrade to "warn" with no team
	// colour. Empty state is handled by the parent.
	import { CrownIcon, TrophyIcon, MinusIcon } from '@lucide/svelte';
	import type { GameData } from '$lib/types/scraper';
	import StatTile, { type StatRow } from '../shared/StatTile.svelte';
	import { teamAccent } from '../shared/util';
	import type { WinnerInfo } from './postgame-vm';

	let {
		winner,
		gameData,
		endedAtStr
	}: {
		winner: WinnerInfo;
		gameData: GameData | null;
		endedAtStr: string;
	} = $props();

	type StatusKind = 'on' | 'off' | 'warn' | 'info' | 'neutral' | 'pointer' | 'none';

	const accent = $derived(
		winner.kind === 'team' && winner.team !== null ? teamAccent(winner.team) : null
	);

	const statusKind = $derived<StatusKind>(
		winner.kind === 'tie' ? 'warn' : winner.kind === 'none' ? 'neutral' : 'on'
	);

	const displayValue = $derived(winner.kind === 'none' ? '—' : `${winner.name} · ${winner.score}`);

	const subtext = $derived.by(() => {
		if (winner.kind === 'none') return undefined;
		if (winner.runnerUpScore === null) return undefined;
		const margin = winner.score - winner.runnerUpScore;
		if (winner.kind === 'tie') return `tied at ${winner.score}`;
		if (margin <= 0) return `runner-up ${winner.runnerUpScore}`;
		return `+${margin} over runner-up (${winner.runnerUpScore})`;
	});

	const bodyRows = $derived.by<StatRow[]>(() => {
		if (!gameData) return [];
		const rows: StatRow[] = [];
		if (gameData.map) {
			rows.push({ label: 'map', display: gameData.map, statusKind: 'neutral' });
		}
		const variant = gameData.variant_name || gameData.gametype || '';
		if (variant) {
			rows.push({ label: 'gametype', display: variant, statusKind: 'neutral' });
		}
		rows.push({
			label: 'ended',
			display: endedAtStr,
			statusKind: endedAtStr === 'never' || endedAtStr === '—' ? 'off' : 'neutral'
		});
		return rows;
	});
</script>

{#snippet headlineIcon()}
	{#if winner.kind === 'team' || winner.kind === 'ffa'}
		<CrownIcon class="size-4" />
	{:else if winner.kind === 'tie'}
		<MinusIcon class="size-4" />
	{:else}
		<TrophyIcon class="size-4" />
	{/if}
{/snippet}

<div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
	<!-- Tall hero tile: the winning-team banner. Spans two columns on >sm so
		the colored dot + large text reads first. -->
	<div class="sm:col-span-2">
		<div class="flex h-full items-center gap-3 card preset-tonal p-4 {accent ? accent.bg : ''}">
			<div
				class="flex size-10 shrink-0 items-center justify-center rounded-full {accent
					? accent.dot
					: 'bg-surface-300-700'} text-surface-50-950"
			>
				{@render headlineIcon()}
			</div>
			<div class="min-w-0 flex-1">
				<div class="text-surface-700-200 text-[10px] font-semibold tracking-wide uppercase">
					{winner.kind === 'tie' ? 'Result' : 'Winner'}
				</div>
				<div class="truncate text-2xl font-bold tabular-nums">
					{displayValue}
				</div>
				{#if subtext}
					<div class="text-surface-700-200 text-xs tabular-nums">{subtext}</div>
				{/if}
			</div>
		</div>
	</div>
	<StatTile label="match" rows={bodyRows} {statusKind} />
</div>
