// Shared v2 → v1 GameData adapter — used by the overview and postgame
// view-models to keep their v1-shape section components rendering while
// the rest of the debug page migrates off v1 types. Lives in shared/
// (not under any one tab's folder) because both the game tab and the
// postgame tab depend on it; the game tab itself no longer uses the
// adapter directly (its rewrite reads v2 GamePayload).
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

import type { GamePayload, ScenarioPayload } from '$lib/types/scraper-v2';
import type {
	GameData,
	PowerItemSpawn,
	StaticCachePtrs,
	StaticFog,
	StaticPlayerSpawn
} from '$lib/types/scraper';

export function v2ToV1GameData(
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
			gametype_mask: p.gametype_mask ?? 0,
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
		// v2 capture has no explicit local_player_count; derive it from the
		// roster's local players (those with a local_index).
		local_count: game?.players?.filter((p) => p.local_index != null).length ?? 0,
		power_item_spawns: powerItemSpawns,
		machines: game?.machines ?? null,
		game_difficulty: scenario?.game_difficulty ?? 0,
		player_spawns: playerSpawns,
		fog,
		object_types: scenario?.object_types ?? null,
		tag_cache: tagCache
	};
}
