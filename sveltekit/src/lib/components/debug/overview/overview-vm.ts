// View-model for the overview tab. Sources the v2 store (per-class
// payloads from PR 20) and projects into the v1 shapes the section
// components still consume:
//   - gameData ← v2ToV1GameData(game, scenario)
//   - tick     ← v2ToV1Tick(tick, objects, game)
//   - events   ← v2 AnyEvent[] wrapped in v1 Envelope shape
//   - previousGame ← v2ToV1PreviousGame(previous_game)
//
// Plain-TS so reactivity is established at the call site by wrapping
// in $derived.by().

import { scraperWSV2 } from '$lib/stores/scraper-ws-v2.svelte';
import type { DebugContext } from '../context';
import type {
	Envelope,
	GameData,
	GamePlayer,
	Phase,
	PreviousGameInfo,
	StateInputs,
	TeamScore,
	TickPayload
} from '$lib/types/scraper';
import type { AnyEvent } from '$lib/types/scraper-v2';
import { v2ToV1GameData } from '../shared/v2-to-v1-game';
import { v2ToV1Tick } from '../tick/tick-vm';
import { v2ToV1PreviousGame } from '../postgame/postgame-vm';
import { teamAccent, teamLabel } from '../shared/util';

type ScraperWSV2 = typeof scraperWSV2;

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

// Wrap a v2 event payload as a v1 Envelope so RecentEvents/EventTile
// (already migrated in PR 25 to v2-aware summarizeEvent) keeps reading
// env.payload.event_type / env.tick.
function v2EventToV1(ev: AnyEvent, instance: string): Envelope {
	return {
		v: 2,
		type: 'event',
		instance,
		tick: ev.tick,
		payload: ev as unknown as Record<string, unknown>
	};
}

export function buildOverviewVm(name: string, ws: ScraperWSV2, ctx: DebugContext): OverviewVm {
	const v2Game = ws.game[name] ?? null;
	const v2Scenario = ws.scenario[name] ?? null;
	const v2Tick = ws.tick[name] ?? null;
	const v2Objects = ws.objects[name] ?? null;
	const v2Prev = ws.previousGame[name] ?? null;
	const v2Events = ws.events[name] ?? [];

	const projectedGame = v2ToV1GameData(v2Game, v2Scenario);
	const gameData = projectedGame ?? ctx.inspect?.game_data ?? null;

	const projectedTick = v2ToV1Tick(v2Tick, v2Objects, v2Game);
	const tick = projectedTick ?? ctx.inspect?.latest_tick ?? null;

	const events: Envelope[] =
		v2Events.length > 0
			? v2Events.map((ev) => v2EventToV1(ev, name))
			: (ctx.inspect?.recent_events ?? []);

	// state_inputs is part of the debug class on v2 (alongside score_probe).
	// The debug tab subscribes to it; the overview tab keeps the inspect
	// fallback so the diagnostic grid renders without forcing a debug-class
	// subscription on the overview page.
	const stateInputs =
		(ws.debug[name]?.state_inputs as StateInputs | undefined) ?? ctx.inspect?.state_inputs ?? null;

	const phase: Phase = ((v2Game?.phase ?? ctx.inspect?.phase) as Phase | undefined) ?? 'idle';

	const projectedPrev = v2ToV1PreviousGame(v2Prev);
	const previousGame = projectedPrev ?? ctx.inspect?.previous_game ?? null;

	const gameDataAtMs = ws.gameAt[name] ?? ws.scenarioAt[name] ?? ctx.inspectAt;
	const tickAtMs = ws.tickAt[name];
	const lastReadAtMs = parseIsoMs(v2Game?.last_read_at ?? ctx.inspect?.last_read_at);
	const previousGameEndedAtMs = parseIsoMs(previousGame?.ended_at);
	const tickValue = ws.engineTick[name] ?? ctx.inspect?.tick;
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
		lastEventsReplyPhase: ws.lastEventsReplyPhase[name] as Phase | undefined
	};
}
