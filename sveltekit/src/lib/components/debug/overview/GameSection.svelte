<script lang="ts">
	// Game-context tile grid: map / gametype / mode / time / variant + two
	// merged tiles (Limits = score+time limit, Engine = machines+difficulty).
	// Split out of MatchHeader so OverviewTab can wrap it in its own
	// accordion item independent of the leaderboard.

	import {
		ClockIcon,
		CpuIcon,
		Gamepad2Icon,
		MapIcon,
		TagIcon,
		TargetIcon,
		UsersIcon
	} from '@lucide/svelte';
	import type { GameData } from '$lib/types/scraper';
	import StatTile, { type StatRow } from '../shared/StatTile.svelte';
	import { ENGINE_HZ, fmtScoreLimit, fmtTickAsTime, fmtTicksAsDuration } from '../shared/util';

	let {
		gameData,
		tickValue,
		showHeader = true
	}: {
		gameData: GameData;
		tickValue: number | undefined;
		showHeader?: boolean;
	} = $props();

	const isTeam = $derived(gameData.is_team_game === true);
	const limitTicks = $derived(gameData.time_limit_ticks ?? 0);

	type StatusKind = 'on' | 'off' | 'warn' | 'info' | 'neutral' | 'pointer' | 'none';

	const timeTile = $derived.by<{ display: string; statusKind: StatusKind }>(() => {
		if (tickValue === undefined || tickValue <= 0) {
			return { display: '—', statusKind: 'none' };
		}
		if (limitTicks > 0) {
			const remaining = Math.max(0, limitTicks - tickValue);
			if (remaining <= 0) return { display: 'expired', statusKind: 'off' };
			const lowSeconds = remaining < 30 * ENGINE_HZ;
			return {
				display: `${fmtTickAsTime(remaining)} left`,
				statusKind: lowSeconds ? 'off' : 'on'
			};
		}
		return { display: fmtTickAsTime(tickValue), statusKind: 'on' };
	});

	const variant = $derived(gameData.variant_name || '');
	const scoreLimitStr = $derived(fmtScoreLimit(gameData.score_limit));
	const timeLimitStr = $derived(fmtTicksAsDuration(gameData.time_limit_ticks));
	const machineCount = $derived(gameData.machines?.length ?? 0);
	const difficulty = $derived(gameData.game_difficulty ?? 0);

	const limitsRows = $derived<StatRow[]>([
		{
			label: 'score',
			display: scoreLimitStr,
			statusKind: scoreLimitStr === 'none' ? 'neutral' : 'on'
		},
		{
			label: 'time',
			display: timeLimitStr,
			statusKind: timeLimitStr === 'none' ? 'neutral' : 'on'
		}
	]);

	const engineRows = $derived<StatRow[]>([
		{
			label: 'machines',
			display: String(machineCount),
			statusKind: machineCount > 0 ? 'on' : 'neutral'
		},
		{
			label: 'difficulty',
			display: String(difficulty),
			statusKind: difficulty > 0 ? 'on' : 'neutral',
			title:
				'0=Easy 1=Normal 2=Heroic 3=Legendary (vestigial in MP — value is whatever the engine last set)'
		}
	]);
</script>

{#snippet mapIcon()}<MapIcon class="size-3.5" />{/snippet}
{#snippet gametypeIcon()}<Gamepad2Icon class="size-3.5" />{/snippet}
{#snippet modeIcon()}<UsersIcon class="size-3.5" />{/snippet}
{#snippet timeIcon()}<ClockIcon class="size-3.5" />{/snippet}
{#snippet variantIcon()}<TagIcon class="size-3.5" />{/snippet}
{#snippet limitsIcon()}<TargetIcon class="size-3.5" />{/snippet}
{#snippet engineIcon()}<CpuIcon class="size-3.5" />{/snippet}

<section>
	{#if showHeader}
		<div class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">Game</div>
	{/if}
	<div class="grid grid-cols-2 gap-2 sm:grid-cols-3 md:grid-cols-4">
		<StatTile
			label="map"
			display={gameData.map || '—'}
			statusKind={gameData.map ? 'on' : 'none'}
			title={gameData.map}
			icon={mapIcon}
		/>
		<StatTile
			label="gametype"
			display={gameData.gametype || '—'}
			statusKind={gameData.gametype ? 'on' : 'none'}
			title={gameData.gametype}
			icon={gametypeIcon}
		/>
		<StatTile
			label="mode"
			display={isTeam ? 'team' : 'ffa'}
			statusKind={isTeam ? 'on' : 'neutral'}
			icon={modeIcon}
		/>
		<StatTile
			label="time"
			display={timeTile.display}
			statusKind={timeTile.statusKind}
			icon={timeIcon}
		/>
		<StatTile
			label="variant"
			display={variant || '—'}
			statusKind="none"
			title={variant}
			icon={variantIcon}
		/>
		<StatTile label="limits" rows={limitsRows} icon={limitsIcon} />
		<StatTile label="engine" rows={engineRows} icon={engineIcon} />
	</div>
</section>
