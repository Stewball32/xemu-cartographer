// Sample-data mode for the OBS overlays. Lets Stewart preview the overlays
// rendering with a believable 4v4 Team Slayer on Bloodgulch — no live xemu, no
// minted token. Enabled with ?mock=1 on any overlay URL.
//
// The functions take an optional `frame` counter (the overlay feed ticks it a
// few times a second in mock mode) so health bars wobble and the score creeps
// upward — the preview looks alive instead of frozen, which also visually
// confirms the overlay reacts to feed updates.

import type {
	GamePayload,
	ScenarioPayload,
	TickPayloadV2,
	ObjectsPayload
} from '$lib/types/scraper-v2';

/** Instance name the mock pretends to be. Any instance segment works in mock
 * mode — the URL's `[instance]` is ignored when ?mock=1 is set. */
export const MOCK_INSTANCE = 'demo';

interface MockSeed {
	index: number;
	name: string;
	team: number;
	kills: number;
	deaths: number;
	assists: number;
	score: number;
	isLocal: boolean;
	/** base health/shields as a 0..1 fraction; the frame counter wobbles it. */
	baseHealth: number;
	baseShields: number;
	/** when set, the player is dead and respawning (overrides health/shields). */
	dead?: boolean;
}

const SEEDS: MockSeed[] = [
	{
		index: 0,
		name: 'Stewball',
		team: 0,
		kills: 17,
		deaths: 9,
		assists: 4,
		score: 17,
		isLocal: true,
		baseHealth: 0.82,
		baseShields: 1
	},
	{
		index: 1,
		name: 'gravemind',
		team: 0,
		kills: 12,
		deaths: 11,
		assists: 6,
		score: 12,
		isLocal: true,
		baseHealth: 0.55,
		baseShields: 0.4
	},
	{
		index: 2,
		name: 'noble_six',
		team: 0,
		kills: 9,
		deaths: 13,
		assists: 8,
		score: 9,
		isLocal: false,
		baseHealth: 1,
		baseShields: 0.75
	},
	{
		index: 3,
		name: 'CmdrKeyes',
		team: 0,
		kills: 7,
		deaths: 10,
		assists: 3,
		score: 7,
		isLocal: false,
		baseHealth: 0,
		baseShields: 0,
		dead: true
	},
	{
		index: 4,
		name: 'TheArbiter',
		team: 1,
		kills: 15,
		deaths: 10,
		assists: 5,
		score: 15,
		isLocal: false,
		baseHealth: 0.9,
		baseShields: 0.9
	},
	{
		index: 5,
		name: 'TartarusX',
		team: 1,
		kills: 11,
		deaths: 12,
		assists: 2,
		score: 11,
		isLocal: false,
		baseHealth: 0.3,
		baseShields: 0
	},
	{
		index: 6,
		name: 'Regret',
		team: 1,
		kills: 8,
		deaths: 9,
		assists: 7,
		score: 8,
		isLocal: false,
		baseHealth: 0.65,
		baseShields: 0.6
	},
	{
		index: 7,
		name: 'flood_carrier',
		team: 1,
		kills: 6,
		deaths: 14,
		assists: 1,
		score: 6,
		isLocal: false,
		baseHealth: 1,
		baseShields: 1
	}
];

function clamp01(n: number): number {
	return n < 0 ? 0 : n > 1 ? 1 : n;
}

export function mockGame(frame = 0): GamePayload {
	// Score creeps every ~5s (the feed ticks ~5 fps) so the strip looks live.
	const redDrift = Math.floor(frame / 40);
	const blueDrift = Math.floor((frame + 17) / 47);
	return {
		phase: 'live',
		started_at: '2026-06-20T02:00:00Z',
		last_read_at: '2026-06-20T02:08:30Z',
		engine_tick: 14400 + frame * 6,
		iterations: 1,
		config: {
			gametype: 'slayer',
			variant_name: 'Team Slayer',
			is_team_game: true,
			score_limit: 50,
			time_limit_ticks: 0
		},
		team_scores: [
			{ team: 0, score: 31 + redDrift },
			{ team: 1, score: 27 + blueDrift }
		],
		players: SEEDS.map((s) => ({
			index: s.index,
			name: s.name,
			team: s.team,
			armor_color: s.team,
			score: s.score + (s.team === 0 ? redDrift : blueDrift),
			kills: s.kills + (s.index === 0 ? redDrift : 0),
			deaths: s.deaths,
			assists: s.assists,
			ctf_score: 0,
			team_kills: 0,
			suicides: 0,
			kill_streak: s.index === 0 ? 3 : 0,
			multikill: 0,
			shots_fired: 0,
			shots_hit: 0,
			is_local: s.isLocal,
			local_index: s.isLocal ? s.index : null,
			machine_index: s.team,
			controller_index: s.isLocal ? s.index : null
		})),
		machines: [
			{ index: 0, name: 'RED-XBOX', is_local: true },
			{ index: 1, name: 'BLU-XBOX', is_local: false }
		],
		network: null
	};
}

function mockTickPlayer(s: MockSeed, frame: number): TickPayloadV2['players'][number] {
	// Per-player phase offset so the bars don't all pulse in lockstep.
	const wobble = Math.sin((frame + s.index * 3) / 6) * 0.12;
	const alive = !s.dead;
	const health = alive ? clamp01(s.baseHealth + wobble) : 0;
	const shields = alive ? clamp01(s.baseShields + wobble) : 0;
	return {
		index: s.index,
		alive,
		respawn_in_ticks: s.dead ? Math.max(0, 90 - (frame % 120)) : null,
		pos: { x: 0, y: 0, z: 0 },
		vel: { x: 0, y: 0, z: 0 },
		aim: { x: 1, y: 0, z: 0 },
		zoom_level: 0,
		crouch_scale: 0,
		// Emit the engine's 0..1 fraction, matching the live wire (health/shields
		// at 0x90/0x94 are RUNTIME-VERIFIED 0..1, not 0..75). `health`/`shields`
		// here are already clamped 0..1 above. Max* are the absolute ceilings.
		health,
		shields,
		max_health: 75,
		max_shields: 75,
		has_camo: s.index === 0 && frame % 80 < 30,
		has_overshield: s.index === 4,
		frags: 2,
		plasmas: 1,
		selected_weapon_slot: 0,
		biped_tag: 'characters\\cyborg\\cyborg',
		actions: {
			crouching: false,
			jumping: false,
			firing: frame % 20 < 6 && s.index % 2 === 0,
			flashlight: false,
			throwing_grenade: false,
			meleeing: false,
			using: false
		},
		weapons: [
			{
				slot: 0,
				tag:
					s.index % 2 === 0 ? 'weapons\\assault rifle\\assault rifle' : 'weapons\\pistol\\pistol',
				object_id: 1000 + s.index,
				ammo_mag: 32 - (frame % 32),
				ammo_pack: 96,
				charge: null,
				heat: 0,
				reloading: false
			}
		]
	};
}

export function mockTick(frame = 0): TickPayloadV2 {
	return {
		players: SEEDS.map((s) => mockTickPlayer(s, frame)),
		power_items: [],
		ctf_flags: [],
		game_globals: {
			map_loaded: 1,
			active: 1,
			game_loading_in_progress: 0,
			precache_map_status: 1,
			stored_global_random: 0
		},
		locals: []
	};
}

export function mockScenario(): ScenarioPayload {
	return {
		map: 'levels\\test\\bloodgulch\\bloodgulch',
		game_difficulty: 2,
		fog: null,
		memory_regions: null,
		object_types: [],
		player_spawns: [],
		power_item_spawns: [],
		tag_defs: {}
	};
}

export function mockObjects(): ObjectsPayload {
	return { objects: [], projectiles: [] };
}
