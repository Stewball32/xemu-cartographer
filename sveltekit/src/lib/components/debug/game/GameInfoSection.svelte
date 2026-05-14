<script lang="ts">
	// Game-info tile grid for the Game tab. Each tile combines related
	// wire fields under one human label; every row inside still names the
	// JSON variable it came from so renaming-as-display doesn't hide what
	// the scraper is actually reading. Tile groupings:
	//   - Settings   map (+ basename context) + variant_name
	//   - Gametype   gametype + is_team_game (with team/ffa context)
	//   - Limits     score_limit + time_limit_ticks (with mm:ss context)
	//   - Difficulty game_difficulty (with Easy/Normal/Heroic/Legendary)
	//   - Progress   engine_tick (with mm:ss derived from ENGINE_HZ).
	//                Lives here for fields that move as the match moves —
	//                future-proofed for elapsed/remaining/percent values.
	//
	// No section-level header line: any value shown at the top of the
	// section needs to map to a real wire field, and the previous
	// "game_data captured: 5s ago" reading was a client-side relative
	// time, not something the scraper emits. If freshness ever becomes
	// debug-critical we can add a tile sourced from an actual wire
	// timestamp (last_read_at on the snapshot).

	import type { GameData } from '$lib/types/scraper';
	import FieldTile from '../shared/FieldTile.svelte';
	import { fmtTickAsTime, fmtTicksAsDuration } from '../shared/util';

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

	// Halo's map field is usually a path like `levels\test\bloodgulch\bloodgulch`
	// (or with forward slashes). Surface the basename as context so the operator
	// can read the map name without the path noise, while keeping the raw value
	// visible.
	function lastPathSegment(s: string | undefined | null): string {
		if (!s) return '';
		const i = Math.max(s.lastIndexOf('/'), s.lastIndexOf('\\'));
		return i >= 0 ? s.slice(i + 1) : s;
	}
	const mapShort = $derived(lastPathSegment(gameData.map));

	// Progress context — flips direction based on whether the gametype has a
	// time cap. With a limit the operator wants to see time remaining (↓);
	// without one, time elapsed (↑). Both readings derive from engine_tick,
	// which is uptime-since-boot, not match-relative — once match-start tick
	// is surfaced this can subtract that for a match-correct delta. Until
	// then the remaining branch clamps at 0 so we don't print negative
	// durations when uptime has already blown past the limit.
	function progressContext(
		engineTick: number | undefined,
		timeLimitTicks: number | undefined | null
	): string | undefined {
		if (engineTick === undefined) return undefined;
		if (timeLimitTicks && timeLimitTicks > 0) {
			const remaining = Math.max(0, timeLimitTicks - engineTick);
			return `↓ ${fmtTickAsTime(remaining)}`;
		}
		return `↑ ${fmtTickAsTime(engineTick)}`;
	}

	// Difficulty enum → human-readable name. Vestigial in MP — whatever the
	// engine last set carries over.
	function difficultyName(d: number | undefined): string {
		switch (d) {
			case 0:
				return 'Easy';
			case 1:
				return 'Normal';
			case 2:
				return 'Heroic';
			case 3:
				return 'Legendary';
			default:
				return '—';
		}
	}
</script>

<section>
	{#if showHeader}
		<div class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			Game Info
		</div>
	{/if}

	<div class="grid grid-cols-2 gap-2 sm:grid-cols-3 md:grid-cols-4">
		<FieldTile
			label="Settings"
			rows={[
				{
					source: 'map',
					value: gameData.map,
					context: mapShort && mapShort !== gameData.map ? mapShort : undefined
				},
				{ source: 'variant_name', value: gameData.variant_name }
			]}
		/>
		<FieldTile
			label="Gametype"
			rows={[
				{ source: 'gametype', value: gameData.gametype },
				{
					source: 'is_team_game',
					value: gameData.is_team_game,
					context: isTeam ? 'team' : 'ffa',
					statusKind: isTeam ? 'on' : 'neutral'
				}
			]}
		/>
		<FieldTile
			label="Limits"
			rows={[
				{ source: 'score_limit', value: gameData.score_limit },
				{
					source: 'time_limit_ticks',
					value: gameData.time_limit_ticks,
					context: fmtTicksAsDuration(gameData.time_limit_ticks)
				}
			]}
			title="score_limit may carry a tick count in Oddball / KOTH variants — interpret per-gametype before treating it as a kill count."
		/>
		<FieldTile
			label="Progress"
			rows={[
				{
					source: 'engine_tick',
					value: tickValue,
					context: progressContext(tickValue, gameData.time_limit_ticks),
					title:
						'engine_tick is uptime-since-boot, not match-relative — context counts ↓ when time_limit_ticks > 0 (remaining, clamped at 0), else ↑ (elapsed). Will be match-accurate once match-start tick is surfaced.'
				}
			]}
		/>
		<FieldTile
			label="Difficulty"
			source="game_difficulty"
			value={gameData.game_difficulty}
			context={difficultyName(gameData.game_difficulty)}
			title="0=Easy 1=Normal 2=Heroic 3=Legendary (vestigial in MP — value is whatever the engine last set)"
		/>
	</div>
</section>
