// View-model for the Game tab: shapes the static per-match GameData (read once
// per match, unchanged until the postgame transition) into a single object the
// GameTab and its children can render. Plain-TS so reactivity is established at
// the call site by wrapping in $derived.by() — mirrors overview/overview-vm.ts.
//
// Source preference: prefer the live WS gameData snapshot, fall back to the
// REST inspect snapshot so the tab still renders for callers without an open
// WebSocket.

import { scraperWS } from '$lib/stores/scraper-ws.svelte';
import type { DebugContext } from '../context';
import type {
	GameData,
	GameMachine,
	GamePlayer,
	PowerItemSpawn,
	StaticFog,
	StaticPlayerSpawn,
	TeamScore
} from '$lib/types/scraper';
import { teamAccent, teamLabel } from '../shared/util';

type ScraperWS = typeof scraperWS;

export type GameScoreRow = {
	label: string;
	value: number;
	limit: number;
	accent: { bg: string; dot: string };
};

export type GameVm = {
	gameData: GameData | null;
	tickValue: number | undefined;
	scoreRows: GameScoreRow[];
	players: GamePlayer[];
	playerSpawns: StaticPlayerSpawn[];
	powerItemSpawns: PowerItemSpawn[];
	machines: GameMachine[];
	fog: StaticFog | null;
};

export function buildScoreRows(gameData: GameData | null): GameScoreRow[] {
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

export function buildGameVm(name: string, ws: ScraperWS, ctx: DebugContext): GameVm {
	const gameData = ws.gameData[name] ?? ctx.inspect?.game_data ?? null;
	const tickValue = ws.tickNumbers[name] ?? ctx.inspect?.tick;

	return {
		gameData,
		tickValue,
		scoreRows: buildScoreRows(gameData),
		players: gameData?.players ?? [],
		playerSpawns: gameData?.player_spawns ?? [],
		powerItemSpawns: gameData?.power_item_spawns ?? [],
		machines: gameData?.machines ?? [],
		fog: gameData?.fog ?? null
	};
}
