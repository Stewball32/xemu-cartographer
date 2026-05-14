<script lang="ts">
	// PlayerStatsCard — one card per player, surfacing all 19 GamePlayer wire
	// fields grouped into FieldTile clusters: Identity / Score / Negatives /
	// Streaks / Accuracy. name + index sit in the card header. accuracy is the
	// one derived value (computed in buildPlayerTotalRow) — it rides along in
	// the Accuracy tile next to the raw shots_fired / shots_hit it's derived
	// from.
	//
	// Scope: GamePlayer-only. Weapons, grenades, position, health and the rest
	// of the volatile per-tick state are TickPlayer data and live on the Tick
	// tab (PlayersRuntimeSection → PlayerDetailPanel). Team / armor tinting is
	// the parent's responsibility (PlayersSection / PlayerTotalsSection wrap
	// the card), matching the existing PlayerCard pattern.

	import type { PlayerTotalRow } from '../postgame/postgame-vm';
	import FieldTile, { type FieldRow } from './FieldTile.svelte';
	import { armorLabel } from './util';

	let { row }: { row: PlayerTotalRow } = $props();

	const identityRows = $derived<FieldRow[]>([
		{ source: 'team', value: row.team },
		{ source: 'armor_color', value: row.armor_color, context: armorLabel(row.armor_color) },
		{ source: 'is_local', value: row.is_local, statusKind: row.is_local ? 'on' : 'neutral' },
		{ source: 'local_index', value: row.local_index },
		{ source: 'machine_index', value: row.machine_index },
		{ source: 'controller_index', value: row.controller_index }
	]);

	const scoreRows = $derived<FieldRow[]>([
		{ source: 'score', value: row.score },
		{ source: 'kills', value: row.kills },
		{ source: 'deaths', value: row.deaths },
		{ source: 'assists', value: row.assists },
		{ source: 'ctf_score', value: row.ctf_score }
	]);

	const negativeRows = $derived<FieldRow[]>([
		{ source: 'team_kills', value: row.team_kills },
		{ source: 'suicides', value: row.suicides }
	]);

	const streakRows = $derived<FieldRow[]>([
		{ source: 'kill_streak', value: row.kill_streak },
		{ source: 'multikill', value: row.multikill }
	]);

	const accuracyRows = $derived<FieldRow[]>([
		{ source: 'shots_fired', value: row.shots_fired },
		{ source: 'shots_hit', value: row.shots_hit },
		{ source: 'accuracy', value: row.accuracy }
	]);
</script>

<div class="flex h-full min-w-0 flex-col gap-2 card preset-tonal p-3">
	<header class="flex min-w-0 items-baseline justify-between gap-2">
		<span class="min-w-0 truncate text-sm font-semibold text-surface-900-100">
			{row.name}
		</span>
		<code class="text-surface-500-400 shrink-0 font-mono text-[10px]">#{row.index}</code>
	</header>
	<div class="grid grid-cols-2 gap-2 sm:grid-cols-3">
		<FieldTile label="Identity" rows={identityRows} />
		<FieldTile label="Score" rows={scoreRows} />
		<FieldTile label="Negatives" rows={negativeRows} />
		<FieldTile label="Streaks" rows={streakRows} />
		<FieldTile label="Accuracy" rows={accuracyRows} />
	</div>
</div>
