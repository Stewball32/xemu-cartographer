// View-model for the overview tab: shapes the live scraperWS store + the
// 3s HTTP-poll inspect snapshot into a single object the OverviewTab and its
// children can render. Plain-TS so the file isn't a runes module — the
// reactivity is established at the call site by wrapping in $derived.by().

import { scraperWS } from '$lib/stores/scraper-ws.svelte';
import type { DebugContext } from '../context';
import type {
	Envelope,
	GamePlayer,
	GameData,
	Phase,
	PreviousGameInfo,
	StateInputs,
	TeamScore,
	TickPayload
} from '$lib/types/scraper';
import { teamAccent, teamLabel } from '../shared/util';

type ScraperWS = typeof scraperWS;

export type ScoreRow = {
	label: string;
	value: number;
	limit: number;
	accent: { bg: string; dot: string };
};

export type PlayerScoreTile = {
	name: string;
	score: number;
	team: number;
	kills: number;
	deaths: number;
	assists: number;
	rank: number;
};

export type OverviewVm = {
	gameData: GameData | null;
	tick: TickPayload | null;
	events: Envelope[];
	stateInputs: StateInputs | null;
	phase: Phase;
	previousGame: PreviousGameInfo | null;
	gameDataAtMs: number | undefined;
	tickAtMs: number | undefined;
	lastReadAtMs: number | undefined;
	previousGameEndedAtMs: number | undefined;
	tickValue: number | undefined;
	playerCount: number | undefined;
	scoreRows: ScoreRow[];
	playerScoreTiles: PlayerScoreTile[];
	previousMatchAutoOpen: boolean;
	lastReadStr: string;
	gameDataStr: string;
	tickStr: string;
	previousGameEndedStr: string;
	wsConnected: boolean;
	lastEventsReplyAtMs: number | undefined;
	lastEventsReplyStr: string;
	lastEventsReplyPhase: Phase | undefined;
};

function parseIsoMs(iso: string | undefined | null): number | undefined {
	if (!iso) return undefined;
	const t = Date.parse(iso);
	return Number.isFinite(t) ? t : undefined;
}

export function buildScoreRows(gameData: GameData | null): ScoreRow[] {
	if (!gameData || gameData.is_team_game !== true) return [];
	const limit = gameData.score_limit ?? 0;
	const teamScores: TeamScore[] = gameData.team_scores ?? [];
	return teamScores.map((ts) => ({
		label: teamLabel(ts.team),
		value: ts.score,
		limit,
		accent: teamAccent(ts.team)
	}));
}

// FFA: one mini-tile per player, sorted by score descending. Empty for team games.
export function buildPlayerScoreTiles(gameData: GameData | null): PlayerScoreTile[] {
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

export function buildOverviewVm(name: string, ws: ScraperWS, ctx: DebugContext): OverviewVm {
	const gameData = ws.gameData[name] ?? ctx.inspect?.game_data ?? null;
	const tick = ws.ticks[name] ?? ctx.inspect?.latest_tick ?? null;
	const events = ws.events[name] ?? ctx.inspect?.recent_events ?? [];
	const stateInputs = ctx.inspect?.state_inputs ?? null;
	const phase: Phase = ws.phases[name] ?? ctx.inspect?.phase ?? 'idle';
	const previousGame =
		ws.previousGames[name] !== undefined
			? ws.previousGames[name]
			: (ctx.inspect?.previous_game ?? null);
	const gameDataAtMs = ws.gameDataAt[name] ?? ctx.inspectAt;
	const tickAtMs = ws.ticksAt[name];
	const lastReadAtMs = parseIsoMs(ws.lastReadAt[name] ?? ctx.inspect?.last_read_at);
	const previousGameEndedAtMs = parseIsoMs(previousGame?.ended_at);
	const tickValue = ws.tickNumbers[name] ?? ctx.inspect?.tick;
	const playerCount = gameData?.players?.length;

	return {
		gameData,
		tick,
		events,
		stateInputs,
		phase,
		previousGame,
		gameDataAtMs,
		tickAtMs,
		lastReadAtMs,
		previousGameEndedAtMs,
		tickValue,
		playerCount,
		scoreRows: buildScoreRows(gameData),
		playerScoreTiles: buildPlayerScoreTiles(gameData),
		// Auto-open the previous-match panel when there's no current match —
		// the most useful view when the runner is between matches.
		previousMatchAutoOpen: gameData == null,
		lastReadStr: ctx.relativeTime(lastReadAtMs),
		gameDataStr: ctx.relativeTime(gameDataAtMs),
		tickStr: ctx.relativeTime(tickAtMs),
		previousGameEndedStr: ctx.relativeTime(previousGameEndedAtMs),
		wsConnected: ws.connected,
		lastEventsReplyAtMs: ws.lastEventsReplyAt[name],
		lastEventsReplyStr: ctx.relativeTime(ws.lastEventsReplyAt[name]),
		lastEventsReplyPhase: ws.lastEventsReplyPhase[name]
	};
}
