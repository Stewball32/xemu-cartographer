// Per-player end-of-match totals — a faithful superset of the v1 GamePlayer
// wire struct plus the derived `accuracy` string. Originally lived under
// debug/postgame/ but PlayerStatsCard (shared) plus the Game tab's
// PlayersSection all depend on it, so it moved to debug/shared/ when the
// postgame folder was retired in favour of the envelope-class
// `previous_game` tab (M6c).
//
// Pure TS; reactivity is wired at the call site via $derived.by().

import type { GameData, GamePlayer } from '$lib/types/scraper';
import { fmtPct } from './util';

export type PlayerTotalRow = {
	index: number;
	name: string;
	team: number;
	armor_color: number;
	// Identity-ish fields the live caller cares about (postgame snapshots
	// preserve them too, even though they aren't useful end-of-match — the
	// debug page just wants every wire field rendered somewhere).
	ctf_score: number;
	is_local: boolean | null;
	local_index: number | null;
	machine_index: number | null;
	controller_index: number | null;
	score: number;
	kills: number;
	deaths: number;
	assists: number;
	team_kills: number;
	suicides: number;
	kill_streak: number;
	multikill: number;
	shots_fired: number;
	shots_hit: number;
	accuracy: string;
};

export function buildPlayerTotalRow(p: GamePlayer): PlayerTotalRow {
	const shotsFired = p.shots_fired ?? 0;
	const shotsHit = p.shots_hit ?? 0;
	return {
		index: p.index,
		name: p.name || '—',
		team: p.team,
		armor_color: p.armor_color,
		ctf_score: p.ctf_score ?? 0,
		is_local: p.is_local ?? null,
		local_index: p.local_index ?? null,
		machine_index: p.machine_index ?? null,
		controller_index: p.controller_index ?? null,
		score: p.score ?? 0,
		kills: p.kills ?? 0,
		deaths: p.deaths ?? 0,
		assists: p.assists ?? 0,
		team_kills: p.team_kills ?? 0,
		suicides: p.suicides ?? 0,
		kill_streak: p.kill_streak ?? 0,
		multikill: p.multikill ?? 0,
		shots_fired: shotsFired,
		shots_hit: shotsHit,
		accuracy: shotsFired > 0 ? fmtPct(shotsHit / shotsFired) : '—'
	};
}

export function buildPlayerTotals(gameData: GameData | null): {
	byTeam: Array<{ team: number; rows: PlayerTotalRow[] }>;
	flat: PlayerTotalRow[];
} {
	const rows = (gameData?.players ?? []).map(buildPlayerTotalRow);
	if (!gameData || gameData.is_team_game !== true) {
		return { byTeam: [], flat: rows.sort((a, b) => b.score - a.score) };
	}
	const flat = rows.sort((a, b) => {
		if (a.team !== b.team) return a.team - b.team;
		return b.score - a.score;
	});
	const groups = new Map<number, PlayerTotalRow[]>();
	for (const row of flat) {
		const arr = groups.get(row.team) ?? [];
		arr.push(row);
		groups.set(row.team, arr);
	}
	const byTeam = [...groups.entries()]
		.sort(([a], [b]) => a - b)
		.map(([team, members]) => ({ team, rows: members }));
	return { byTeam, flat };
}
