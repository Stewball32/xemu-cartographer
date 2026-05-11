<script lang="ts">
	// Per-player end-of-match stats. In team games, one ColGroupedTable per
	// team (with a colored team-label header bar). In FFA, a single
	// ColGroupedTable. Columns: name, kills, deaths, assists, team_kills,
	// suicides, kill_streak, multikill, shots_fired, shots_hit, accuracy.
	// The name cell carries armor-tinted text (FFA) or team-tinted (team).
	import type { ColGroup } from '../shared/col-grouped-table';
	import ColGroupedTable from '../shared/ColGroupedTable.svelte';
	import { armorTextClass, teamAccent, teamLabel } from '../shared/util';
	import type { PlayerTotalRow } from './postgame-vm';

	let {
		playerTotalsByTeam,
		playerTotalsFlat,
		isTeamGame,
		showHeader = true
	}: {
		playerTotalsByTeam: Array<{ team: number; rows: PlayerTotalRow[] }>;
		playerTotalsFlat: PlayerTotalRow[];
		isTeamGame: boolean;
		showHeader?: boolean;
	} = $props();

	// Column groups — identity first, then combat counters, then accuracy.
	const groups: ColGroup[] = [
		{
			label: 'Player',
			columns: [{ key: 'name', label: 'name' }]
		},
		{
			label: 'Score',
			columns: [
				{ key: 'score', label: 'pts' },
				{ key: 'kills', label: 'K' },
				{ key: 'deaths', label: 'D' },
				{ key: 'assists', label: 'A' }
			]
		},
		{
			label: 'Negatives',
			columns: [
				{ key: 'team_kills', label: 'TK' },
				{ key: 'suicides', label: 'suicides' }
			]
		},
		{
			label: 'Streaks',
			columns: [
				{ key: 'kill_streak', label: 'streak' },
				{ key: 'multikill', label: 'multi' }
			]
		},
		{
			label: 'Accuracy',
			columns: [
				{ key: 'shots_fired', label: 'fired' },
				{ key: 'shots_hit', label: 'hit' },
				{ key: 'accuracy', label: 'acc' }
			]
		}
	];

	// ColGroupedTable renders scalar cells only; colour cues live on the
	// per-team header bar (team games) or the swatch legend below (FFA).
	function toRowDicts(rows: PlayerTotalRow[]): Record<string, unknown>[] {
		return rows.map((r) => ({
			name: r.name,
			score: r.score,
			kills: r.kills,
			deaths: r.deaths,
			assists: r.assists,
			team_kills: r.team_kills,
			suicides: r.suicides,
			kill_streak: r.kill_streak,
			multikill: r.multikill,
			shots_fired: r.shots_fired,
			shots_hit: r.shots_hit,
			accuracy: r.accuracy
		}));
	}

	const ffaRowsDict = $derived(toRowDicts(playerTotalsFlat));

	const hasContent = $derived(
		isTeamGame ? playerTotalsByTeam.length > 0 : playerTotalsFlat.length > 0
	);
</script>

<section>
	{#if showHeader}
		<div class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			Player totals
		</div>
	{/if}
	{#if !hasContent}
		<div class="text-surface-500-400 card preset-tonal p-3 text-sm">
			no player totals captured for the previous match
		</div>
	{:else if isTeamGame}
		<div class="space-y-3">
			{#each playerTotalsByTeam as group (group.team)}
				{@const accent = teamAccent(group.team)}
				<div>
					<div
						class="flex items-center gap-2 rounded-t px-2 py-1 text-xs font-semibold uppercase {accent.bg}"
					>
						<span class="block size-3 rounded-sm {accent.dot}"></span>
						<span>{teamLabel(group.team)}</span>
						<span class="text-surface-700-200 ml-auto font-normal normal-case tabular-nums">
							{group.rows.length} player{group.rows.length === 1 ? '' : 's'}
						</span>
					</div>
					<ColGroupedTable rows={toRowDicts(group.rows)} {groups} stickyFirst />
				</div>
			{/each}
		</div>
	{:else}
		<div>
			{#if playerTotalsFlat.length > 0}
				<div class="mb-1.5 flex flex-wrap gap-2">
					{#each playerTotalsFlat as r (r.index)}
						<span
							class="inline-flex items-center gap-1 text-[11px] {armorTextClass(r.armor_color)}"
						>
							<span class="block size-2 rounded-sm bg-current" title={r.name}></span>
							<span class="font-semibold">{r.name}</span>
						</span>
					{/each}
				</div>
			{/if}
			<ColGroupedTable rows={ffaRowsDict} {groups} stickyFirst />
		</div>
	{/if}
</section>
