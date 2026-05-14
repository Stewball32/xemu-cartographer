// View-model for the postgame tab: shapes the just-ended match snapshot from
// scraperWS.previousGames[name] into a single object PostgameTab and its
// children render. Plain-TS (not a runes module) — reactivity is established
// at the call site via $derived.by(). Mirrors overview/overview-vm.ts.

import { scraperWS } from '$lib/stores/scraper-ws.svelte';
import type { DebugContext } from '../context';
import type {
	Envelope,
	GameData,
	GamePlayer,
	PreviousGameInfo,
	TeamScore
} from '$lib/types/scraper';
import { fmtPct, teamAccent, teamLabel } from '../shared/util';

type ScraperWS = typeof scraperWS;

// One per-player row of end-of-match totals. A faithful superset of the
// GamePlayer wire struct plus the derived `accuracy` string — rendered by
// PlayerStatsCard on both the Game tab (merged players section) and the
// Postgame tab (PlayerTotalsSection).
export type PlayerTotalRow = {
	index: number;
	name: string;
	team: number;
	armor_color: number;
	// Identity-ish fields the live caller cares about (Postgame snapshots
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

// Final-scoreboard tile (team game).
export type FinalScoreRow = {
	label: string;
	value: number;
	limit: number;
	accent: { bg: string; dot: string };
	team: number;
};

// FFA final-scoreboard mini-tile.
export type FinalPlayerScoreTile = {
	name: string;
	score: number;
	team: number;
	kills: number;
	deaths: number;
	assists: number;
	rank: number;
};

// Winner summary for the hero headline. team === null in FFA. name is the
// winning team label ("Red", "Blue") in team games, or the top player's
// gamertag in FFA.
export type WinnerInfo = {
	kind: 'team' | 'ffa' | 'tie' | 'none';
	team: number | null;
	name: string;
	score: number;
	runnerUpScore: number | null;
};

export type PostgameVm = {
	previousGame: PreviousGameInfo | null;
	gameData: GameData | null;
	events: Envelope[];
	endedAtMs: number | undefined;
	endedAtStr: string;
	isTeamGame: boolean;
	winner: WinnerInfo;
	finalScoreRows: FinalScoreRow[];
	finalPlayerScoreTiles: FinalPlayerScoreTile[];
	playerTotalsByTeam: Array<{ team: number; rows: PlayerTotalRow[] }>;
	playerTotalsFlat: PlayerTotalRow[];
};

function parseIsoMs(iso: string | undefined | null): number | undefined {
	if (!iso) return undefined;
	const t = Date.parse(iso);
	return Number.isFinite(t) ? t : undefined;
}

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

export function buildFinalScoreRows(gameData: GameData | null): FinalScoreRow[] {
	if (!gameData || gameData.is_team_game !== true) return [];
	const limit = gameData.score_limit ?? 0;
	const teamScores: TeamScore[] = gameData.team_scores ?? [];
	return [...teamScores]
		.sort((a, b) => (b.score ?? 0) - (a.score ?? 0))
		.map((ts) => ({
			label: teamLabel(ts.team),
			value: ts.score,
			limit,
			accent: teamAccent(ts.team),
			team: ts.team
		}));
}

export function buildFinalPlayerScoreTiles(gameData: GameData | null): FinalPlayerScoreTile[] {
	if (!gameData || gameData.is_team_game === true) return [];
	const players: GamePlayer[] = gameData.players ?? [];
	return [...players]
		.sort((a, b) => (b.score ?? 0) - (a.score ?? 0))
		.map((p, i) => ({
			name: p.name || '—',
			score: p.score ?? 0,
			team: p.team,
			kills: p.kills ?? 0,
			deaths: p.deaths ?? 0,
			assists: p.assists ?? 0,
			rank: i + 1
		}));
}

function buildWinner(gameData: GameData | null): WinnerInfo {
	if (!gameData) return { kind: 'none', team: null, name: '—', score: 0, runnerUpScore: null };
	if (gameData.is_team_game === true) {
		const scores = [...(gameData.team_scores ?? [])].sort(
			(a, b) => (b.score ?? 0) - (a.score ?? 0)
		);
		if (scores.length === 0) {
			return { kind: 'none', team: null, name: '—', score: 0, runnerUpScore: null };
		}
		const top = scores[0];
		const runnerUp = scores[1]?.score ?? null;
		if (runnerUp !== null && runnerUp === top.score) {
			return {
				kind: 'tie',
				team: null,
				name: 'Tie',
				score: top.score,
				runnerUpScore: runnerUp
			};
		}
		return {
			kind: 'team',
			team: top.team,
			name: teamLabel(top.team),
			score: top.score,
			runnerUpScore: runnerUp
		};
	}
	// FFA: top scorer by score.
	const players = [...(gameData.players ?? [])].sort((a, b) => (b.score ?? 0) - (a.score ?? 0));
	if (players.length === 0) {
		return { kind: 'none', team: null, name: '—', score: 0, runnerUpScore: null };
	}
	const top = players[0];
	const runnerUp = players[1]?.score ?? null;
	if (runnerUp !== null && runnerUp === (top.score ?? 0)) {
		return {
			kind: 'tie',
			team: null,
			name: 'Tie',
			score: top.score ?? 0,
			runnerUpScore: runnerUp
		};
	}
	return {
		kind: 'ffa',
		team: top.team,
		name: top.name || `Player ${top.index}`,
		score: top.score ?? 0,
		runnerUpScore: runnerUp
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

export function buildPostgameVm(name: string, ws: ScraperWS, ctx: DebugContext): PostgameVm {
	const previousGame =
		ws.previousGames[name] !== undefined
			? ws.previousGames[name]
			: (ctx.inspect?.previous_game ?? null);
	const gameData = previousGame?.game_data ?? null;
	const events = previousGame?.events ?? [];
	const endedAtMs = parseIsoMs(previousGame?.ended_at);
	const isTeamGame = gameData?.is_team_game === true;
	const totals = buildPlayerTotals(gameData);

	return {
		previousGame,
		gameData,
		events,
		endedAtMs,
		endedAtStr: ctx.relativeTime(endedAtMs),
		isTeamGame,
		winner: buildWinner(gameData),
		finalScoreRows: buildFinalScoreRows(gameData),
		finalPlayerScoreTiles: buildFinalPlayerScoreTiles(gameData),
		playerTotalsByTeam: totals.byTeam,
		playerTotalsFlat: totals.flat
	};
}
