// View-model for the Game tab. Sources data from the v2 game-class +
// scenario-class envelopes (scraperWSV2.game[name] + .scenario[name])
// and projects them back into the v1 GameData shape the existing
// section components (FogSection, PowerItemsSection, SpawnsSection,
// MachinesSection, ObjectTypesSection, TagCacheSection, PlayersSection)
// still consume. Inspect-endpoint fallback preserves the first-paint
// behaviour while the first envelope is in flight.
//
// The v1→v2 shape projection is contained to v2ToV1GameData() below —
// when sub-sections individually migrate to read v2 types directly, the
// adapter shrinks and eventually disappears.
//
// Plain TS; reactivity is wired at the call site via $derived.by().

import { scraperWSV2 } from '$lib/stores/scraper-ws-v2.svelte';
import type { DebugContext } from '../context';
import type { GamePayload, ScenarioPayload } from '$lib/types/scraper-v2';
import type {
	GameData,
	GameMachine,
	GamePlayer,
	PowerItemSpawn,
	StaticCachePtrs,
	StaticFog,
	StaticObjectType,
	StaticPlayerSpawn,
	TeamScore
} from '$lib/types/scraper';
import { teamAccent, teamLabel } from '../shared/util';
import { buildPlayerTotals, type PlayerTotalRow } from '../postgame/postgame-vm';

type ScraperWSV2 = typeof scraperWSV2;

export type GameScoreRow = {
	team: number;
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
	objectTypes: StaticObjectType[];
	tagCache: StaticCachePtrs | null;
	isTeamGame: boolean;
	playerTotalsByTeam: Array<{ team: number; rows: PlayerTotalRow[] }>;
	playerTotalsFlat: PlayerTotalRow[];
};

export function buildScoreRows(gameData: GameData | null): GameScoreRow[] {
	if (!gameData || gameData.is_team_game !== true) return [];
	const limit = gameData.score_limit ?? 0;
	const teamScores: TeamScore[] = gameData.team_scores ?? [];
	return teamScores.map((ts) => ({
		team: ts.team,
		label: teamLabel(ts.team),
		value: ts.score,
		limit,
		accent: teamAccent(ts.team)
	}));
}

// v2ToV1GameData projects v2 game-class + scenario-class payloads into
// the v1 GameData shape the GameTab's section components still expect.
// Returns null when neither v2 envelope has arrived (the tab's empty-
// state UI then shows "waiting for first read").
//
// Shape differences handled here:
//   - v2 GamePayload.config nests gametype/variant_name/is_team_game/
//     score_limit/time_limit_ticks; v1 flattens onto GameData.
//   - v2 ScenarioPowerItemSpawn nests pos and renames spawn_interval_ticks
//     → interval_ticks.
//   - v2 ScenarioPlayerSpawn nests pos and replaces gametype_0..3 + unk_0
//     with a gametypes uint8[] array.
//   - v2 ScenarioFog nests color: {r,g,b}; v1 flattens to color_r/g/b.
//   - v2 ScenarioMemoryRegions nests each region as {base,size}; v1
//     flattens to <region>_base / <region>_size pairs and (mis-)names
//     the holder tag_cache.
function v2ToV1GameData(
	game: GamePayload | null,
	scenario: ScenarioPayload | null
): GameData | null {
	if (!game && !scenario) return null;
	const playerSpawns: StaticPlayerSpawn[] =
		scenario?.player_spawns?.map((s) => ({
			index: s.index,
			x: s.pos.x,
			y: s.pos.y,
			z: s.pos.z,
			facing: s.facing,
			team_index: s.team_index,
			bsp_index: s.bsp_index,
			unk_0: 0,
			gametype_0: s.gametypes?.[0] ?? 0,
			gametype_1: s.gametypes?.[1] ?? 0,
			gametype_2: s.gametypes?.[2] ?? 0,
			gametype_3: s.gametypes?.[3] ?? 0
		})) ?? [];
	const powerItemSpawns: PowerItemSpawn[] =
		scenario?.power_item_spawns?.map((p) => ({
			spawn_id: p.spawn_id,
			tag: p.tag,
			spawn_interval_ticks: p.interval_ticks,
			x: p.pos.x,
			y: p.pos.y,
			z: p.pos.z
		})) ?? [];
	const fog: StaticFog | null = scenario?.fog
		? {
				color_r: scenario.fog.color.r,
				color_g: scenario.fog.color.g,
				color_b: scenario.fog.color.b,
				max_density: scenario.fog.max_density,
				atmo_min_dist: scenario.fog.atmo_min_dist,
				atmo_max_dist: scenario.fog.atmo_max_dist
			}
		: null;
	const tagCache: StaticCachePtrs | null = scenario?.memory_regions
		? {
				game_state_base: scenario.memory_regions.game_state.base,
				game_state_size: scenario.memory_regions.game_state.size,
				tag_cache_base: scenario.memory_regions.tag_cache.base,
				tag_cache_size: scenario.memory_regions.tag_cache.size,
				texture_cache_base: scenario.memory_regions.texture_cache.base,
				texture_cache_size: scenario.memory_regions.texture_cache.size,
				sound_cache_base: scenario.memory_regions.sound_cache.base,
				sound_cache_size: scenario.memory_regions.sound_cache.size
			}
		: null;
	return {
		map: scenario?.map ?? '',
		gametype: game?.config?.gametype ?? '',
		variant_name: game?.config?.variant_name,
		is_team_game: game?.config?.is_team_game ?? false,
		score_limit: game?.config?.score_limit ?? 0,
		time_limit_ticks: game?.config?.time_limit_ticks ?? 0,
		team_scores: game?.team_scores ?? null,
		players: game?.players ?? null,
		power_item_spawns: powerItemSpawns,
		machines: game?.machines ?? null,
		game_difficulty: scenario?.game_difficulty ?? 0,
		player_spawns: playerSpawns,
		fog,
		object_types: scenario?.object_types ?? null,
		tag_cache: tagCache
	};
}

export function buildGameVm(name: string, ws: ScraperWSV2, ctx: DebugContext): GameVm {
	const game = ws.game[name] ?? null;
	const scenario = ws.scenario[name] ?? null;
	const projected = v2ToV1GameData(game, scenario);
	const gameData = projected ?? ctx.inspect?.game_data ?? null;
	const tickValue = ws.engineTick[name] ?? ctx.inspect?.tick;
	const totals = buildPlayerTotals(gameData);

	return {
		gameData,
		tickValue,
		scoreRows: buildScoreRows(gameData),
		players: gameData?.players ?? [],
		playerSpawns: gameData?.player_spawns ?? [],
		powerItemSpawns: gameData?.power_item_spawns ?? [],
		machines: gameData?.machines ?? [],
		fog: gameData?.fog ?? null,
		objectTypes: gameData?.object_types ?? [],
		tagCache: gameData?.tag_cache ?? null,
		isTeamGame: gameData?.is_team_game === true,
		playerTotalsByTeam: totals.byTeam,
		playerTotalsFlat: totals.flat
	};
}
