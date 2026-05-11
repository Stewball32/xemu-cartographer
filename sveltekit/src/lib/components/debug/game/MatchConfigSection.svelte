<script lang="ts">
	// Match-config tile grid for the Game tab. Static per-match fields read once
	// at match start: map, gametype, variant_name, is_team_game, score_limit,
	// time_limit_ticks, game_difficulty. Mirrors overview/GameSection.svelte's
	// StatTile grid style; the time tile here shows the static limit (not
	// remaining-time-on-clock — that's the Tick tab's concern).

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
	import StatTile from '../shared/StatTile.svelte';
	import { fmtScoreLimit, fmtTicksAsDuration } from '../shared/util';

	let {
		gameData,
		showHeader = true
	}: {
		gameData: GameData;
		showHeader?: boolean;
	} = $props();

	const isTeam = $derived(gameData.is_team_game === true);
	const variant = $derived(gameData.variant_name || '');
	const scoreLimitStr = $derived(fmtScoreLimit(gameData.score_limit));
	const timeLimitStr = $derived(fmtTicksAsDuration(gameData.time_limit_ticks));
	const difficulty = $derived(gameData.game_difficulty ?? 0);
</script>

{#snippet mapIcon()}<MapIcon class="size-3.5" />{/snippet}
{#snippet gametypeIcon()}<Gamepad2Icon class="size-3.5" />{/snippet}
{#snippet modeIcon()}<UsersIcon class="size-3.5" />{/snippet}
{#snippet variantIcon()}<TagIcon class="size-3.5" />{/snippet}
{#snippet scoreLimitIcon()}<TargetIcon class="size-3.5" />{/snippet}
{#snippet timeLimitIcon()}<ClockIcon class="size-3.5" />{/snippet}
{#snippet difficultyIcon()}<CpuIcon class="size-3.5" />{/snippet}

<section>
	{#if showHeader}
		<div class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			Match config
		</div>
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
			label="variant"
			display={variant || '—'}
			statusKind={variant ? 'on' : 'none'}
			title={variant}
			icon={variantIcon}
		/>
		<StatTile
			label="score limit"
			display={scoreLimitStr}
			statusKind={scoreLimitStr === 'none' ? 'neutral' : 'on'}
			icon={scoreLimitIcon}
		/>
		<StatTile
			label="time limit"
			display={timeLimitStr}
			statusKind={timeLimitStr === 'none' ? 'neutral' : 'on'}
			icon={timeLimitIcon}
		/>
		<StatTile
			label="difficulty"
			display={String(difficulty)}
			statusKind={difficulty > 0 ? 'on' : 'neutral'}
			title="0=Easy 1=Normal 2=Heroic 3=Legendary (vestigial in MP — value is whatever the engine last set)"
			icon={difficultyIcon}
		/>
	</div>
</section>
