// Sample-data mode for the OBS overlays. Lets Stewart preview the overlays
// rendering with a believable 4v4 Team Slayer on Bloodgulch — no live xemu, no
// minted token. Enabled with ?mock=1 on any overlay URL.
//
// The functions take an optional `frame` counter (the overlay feed ticks it a
// few times a second in mock mode) so health bars wobble and the score creeps
// upward — the preview looks alive instead of frozen, which also visually
// confirms the overlay reacts to feed updates.

import type {
	AnyEvent,
	DeathCause,
	DeathEvent,
	GamePayload,
	ObjectsPayload,
	PlayerRef,
	ScenarioPayload,
	TickPayloadV2
} from '$lib/types/scraper-v2';
import { H2_KEYS, type Appearance } from '$lib/utils/emblem';

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
	/** CE/H2 armor-palette index (0..17) — tints the player card's Spartan. Warm
	 * indices for the red team, cool for blue, so the roster reads team-wise while
	 * every card still shows a distinct hue (real team games lock armor to the
	 * team colour; this is preview data chosen to exercise the palette). */
	armorColor: number;
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
		armorColor: 2, // Red
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
		armorColor: 11, // Orange
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
		armorColor: 16, // Maroon
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
		armorColor: 17, // Salmon
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
		armorColor: 3, // Blue
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
		armorColor: 10, // Cobalt
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
		armorColor: 9, // Cyan
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
		armorColor: 12, // Teal
		baseHealth: 1,
		baseShields: 1
	}
];

function clamp01(n: number): number {
	return n < 0 ? 0 : n > 1 ? 1 : n;
}

// Believable Bloodgulch-ish layout so the top-down VISUALIZER has something to
// render in mock mode. The OBS scoreboard/status overlays ignore positions +
// the objects/scenario-spawn layers, so populating them here is additive. Red
// base sits NW, Blue base SE; players 0 and 4 roam toward mid-map so their dots
// visibly move — the same movement that validates the position offsets live.
// Real Blood Gulch coordinates: the extracted structure-BSP mesh spans
// x[6,132] y[-190,-45] z[-0.3,26], so the mock markers sit INSIDE the real map in
// the 3D visualizer (and the 2D map auto-fits the same layout). Red base near the
// canyon's north mouth, Blue near the south, MID between them; z ~ floor height.
const RED_BASE = { x: 60, y: -70 };
const BLUE_BASE = { x: 70, y: -165 };
const MID = { x: 65, y: -118 };
const FLOOR_Z = 2;

function mockPlayerPos(s: MockSeed, frame: number): { x: number; y: number; z: number } {
	const base = s.team === 0 ? RED_BASE : BLUE_BASE;
	if (s.dead) return { x: base.x, y: base.y, z: FLOOR_Z }; // respawning — sit at base
	if (s.index === 0 || s.index === 4) {
		// Roamer: ping-pong between base and mid-map.
		const t = (Math.sin(frame / 30 + s.index) + 1) / 2;
		return {
			x: base.x + (MID.x - base.x) * t,
			y: base.y + (MID.y - base.y) * t,
			z: FLOOR_Z + 4 * t
		};
	}
	// Orbit near the base so each dot drifts a little.
	const a = frame / 40 + s.index;
	const r = 8 + (s.index % 3) * 4;
	return {
		x: base.x + Math.cos(a) * r,
		y: base.y + Math.sin(a) * r,
		z: FLOOR_Z + (s.index % 2) * 3
	};
}

function mockPlayerAim(s: MockSeed): { x: number; y: number; z: number } {
	// Face the enemy base.
	const target = s.team === 0 ? BLUE_BASE : RED_BASE;
	const base = s.team === 0 ? RED_BASE : BLUE_BASE;
	const dx = target.x - base.x;
	const dy = target.y - base.y;
	const len = Math.hypot(dx, dy) || 1;
	return { x: dx / len, y: dy / len, z: 0 };
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
			armor_color: s.armorColor,
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
		pos: mockPlayerPos(s, frame),
		vel: { x: 0, y: 0, z: 0 },
		aim: mockPlayerAim(s),
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
		// Tracked power items: rockets sit mid-map, the sniper is held by Arbiter
		// (4), the overshield is respawning (counted, off-map), camo is on the
		// floor. spawn_ids line up with mockPowerItemSpawns() for labels.
		power_items: [
			{
				spawn_id: 0,
				status: 'world',
				held_by: null,
				pos: { x: 65, y: -118, z: 2 },
				respawn_in_ticks: null
			},
			{ spawn_id: 1, status: 'held', held_by: 4, pos: null, respawn_in_ticks: null },
			{
				spawn_id: 2,
				status: 'respawning',
				held_by: null,
				pos: null,
				respawn_in_ticks: 300 - (frame % 300)
			},
			{
				spawn_id: 3,
				status: 'world',
				held_by: null,
				pos: { x: 52, y: -148, z: 2 },
				respawn_in_ticks: null
			}
		],
		// Slayer has no flags; the visualizer renders the flag layer live on CTF.
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
		player_spawns: mockPlayerSpawns(),
		power_item_spawns: mockPowerItemSpawns(),
		tag_defs: {}
	};
}

function mockPlayerSpawns(): ScenarioPayload['player_spawns'] {
	const ring = (base: { x: number; y: number }, team: number, start: number) =>
		[0, 1, 2, 3].map((i) => ({
			index: start + i,
			pos: {
				x: base.x + Math.cos((i / 4) * Math.PI * 2) * 10,
				y: base.y + Math.sin((i / 4) * Math.PI * 2) * 10,
				z: 0
			},
			facing: 0,
			team_index: team,
			bsp_index: 0,
			gametypes: []
		}));
	return [...ring(RED_BASE, 0, 0), ...ring(BLUE_BASE, 1, 4)];
}

function mockPowerItemSpawns(): ScenarioPayload['power_item_spawns'] {
	return [
		{
			spawn_id: 0,
			tag: 'weapons\\rocket launcher\\rocket launcher',
			interval_ticks: 3600,
			gametype_mask: 0,
			pos: { x: 65, y: -118, z: 2 }
		},
		{
			spawn_id: 1,
			tag: 'weapons\\sniper rifle\\sniper rifle',
			interval_ticks: 3600,
			gametype_mask: 0,
			pos: { x: 95, y: -100, z: 5 }
		},
		{
			spawn_id: 2,
			tag: 'powerups\\over shield\\over shield',
			interval_ticks: 5400,
			gametype_mask: 0,
			pos: { x: 45, y: -135, z: 5 }
		},
		{
			spawn_id: 3,
			tag: 'powerups\\active camouflage\\active camouflage',
			interval_ticks: 5400,
			gametype_mask: 0,
			pos: { x: 52, y: -148, z: 2 }
		},
		{
			spawn_id: 4,
			tag: 'weapons\\plasma rifle\\plasma rifle',
			interval_ticks: 1800,
			gametype_mask: 0,
			pos: { x: 80, y: -95, z: 1 }
		}
	];
}

// Mock kill feed — a believable rolling Slayer exchange built from the same
// SEEDS roster so names/teams line up with the scoreboard. The feed factory
// ticks `frame` a few times a second; mockEvents derives "how many kills have
// happened so far" from it and returns the most recent ones NEWEST-FIRST, which
// is exactly how the live WS store keeps the per-instance event log — so the
// kill-feed overlay renders identically from either source.

function seedRef(i: number): PlayerRef {
	const s = SEEDS[i];
	return { index: s.index, name: s.name, team: s.team, armor_color: s.team };
}

interface KillStep {
	/** null = no killer (suicide / fall). */
	killer: number | null;
	victim: number;
	weapon: string;
	cause: DeathCause;
}

// One loop of the scripted exchange: cross-team kills, a fall death, a botched
// grenade suicide, and a same-team betrayal — enough variety to exercise every
// kill-feed render branch.
const KILL_SCRIPT: KillStep[] = [
	{ killer: 0, victim: 5, weapon: 'weapons\\pistol\\pistol', cause: 'kill' },
	{ killer: 4, victim: 1, weapon: 'weapons\\sniper rifle\\sniper rifle', cause: 'kill' },
	{ killer: 0, victim: 4, weapon: 'weapons\\assault rifle\\assault rifle', cause: 'kill' },
	{ killer: null, victim: 3, weapon: '', cause: 'fall' },
	{ killer: 6, victim: 0, weapon: 'weapons\\plasma rifle\\plasma rifle', cause: 'kill' },
	{ killer: null, victim: 1, weapon: 'weapons\\frag grenade\\frag grenade', cause: 'suicide' },
	{ killer: 4, victim: 2, weapon: 'weapons\\sniper rifle\\sniper rifle', cause: 'kill' },
	{ killer: 5, victim: 6, weapon: 'weapons\\needler\\needler', cause: 'betrayal' }
];

/** Cadence: a new kill roughly every ~2.4s (12 frames at the ~5 fps mock tick). */
const FRAMES_PER_KILL = 12;
/** Seed a few kills so the preview is populated immediately rather than empty. */
const KILL_BASELINE = 3;

export function mockEvents(frame = 0): AnyEvent[] {
	const total = KILL_BASELINE + Math.floor(Math.max(0, frame) / FRAMES_PER_KILL);
	const show = Math.min(total, 8);
	const out: DeathEvent[] = [];
	for (let k = 0; k < show; k++) {
		const idx = total - 1 - k; // newest first
		const step = KILL_SCRIPT[idx % KILL_SCRIPT.length];
		out.push({
			seq: idx + 1,
			tick: 14400 + idx * 72, // ~2.4s apart at 30 Hz
			at: '2026-06-20T02:08:30Z',
			event_type: 'death',
			victim: seedRef(step.victim),
			victim_pos: { x: 0, y: 0, z: 0 },
			killer: step.killer == null ? null : seedRef(step.killer),
			killer_pos: step.killer == null ? null : { x: 0, y: 0, z: 0 },
			cause: step.cause,
			weapon: step.weapon,
			team_kill: step.cause === 'betrayal',
			respawn_in_ticks: 90
		});
	}
	return out;
}

export function mockObjects(): ObjectsPayload {
	const veh = (id: number, tag: string, x: number, y: number, owner: number | null) => ({
		object_id: id,
		tag,
		type: 1,
		flags: 0,
		pos: { x, y, z: 0 },
		ang_vel: { x: 0, y: 0, z: 0 },
		time_existing: 600,
		owner_unit: owner,
		owner_object: null,
		ultimate_parent: id
	});
	return {
		objects: [
			veh(5001, 'vehicles\\warthog\\warthog', 55, -78, null),
			veh(5002, 'vehicles\\ghost\\ghost', 82, -158, 5), // occupied
			veh(5003, 'vehicles\\banshee\\banshee', 95, -150, null),
			{
				object_id: 5100,
				tag: 'weapons\\assault rifle\\assault rifle',
				type: 2,
				flags: 0,
				pos: { x: 68, y: -110, z: 0 },
				ang_vel: { x: 0, y: 0, z: 0 },
				time_existing: 120,
				owner_unit: null,
				owner_object: null,
				ultimate_parent: 5100
			}
		],
		projectiles: [
			{
				object_id: 6001,
				tag: 'weapons\\frag grenade\\frag grenade',
				pos: { x: 60, y: -115, z: 2 },
				flags: 0,
				action: 0,
				detonation_timer: 1.2,
				distance_traveled: 4
			}
		]
	};
}

// H2 emblem/appearance for the Halo 2-THEMED broadcast previews. Halo 2 players
// carry an emblem (foreground symbol + background plate + four colours) the way
// CE players never did — the H2 broadcast theme renders it on the Spartan's
// chest and as a card badge. Live H2 will source this from the (unfinished) H2
// scraper's profile read; until then, mockAppearance derives a stable,
// distinct-per-slot emblem so the H2 theme is previewable. Deterministic on the
// slot index so a given card looks the same every frame.
//
// NOTE (CE vs H2): CE has NO emblem system — the CE theme intentionally never
// calls this. This is Halo 2 broadcast preview data only.
export function mockAppearance(index: number): Appearance {
	const seed = SEEDS[((index % SEEDS.length) + SEEDS.length) % SEEDS.length];
	const armorPrimary = seed.armorColor % 18;
	return {
		[H2_KEYS.armorPrimary]: armorPrimary,
		[H2_KEYS.armorSecondary]: (armorPrimary + 9) % 18,
		[H2_KEYS.emblemPrimary]: 0, // white symbol
		[H2_KEYS.emblemSecondary]: (armorPrimary + 4) % 18,
		// The Arbiter (slot 4) fittingly rides in as an Elite; everyone else is a
		// Spartan (Master Chief bust).
		[H2_KEYS.character]: index === 4 ? 3 : 0,
		[H2_KEYS.foreground]: (index * 7 + 12) % 64,
		[H2_KEYS.background]: (index * 5 + 3) % 32,
		[H2_KEYS.flags]: 0
	};
}

// Mock profile-avatar table for the broadcast player cards — the ?mock=1 stand-in
// for the live /api/public/profiles endpoint, keyed by lowercased gamertag so it
// matches how the card resolves a live player name. Each seed gets a CE armor
// colour + an H2 emblem/appearance (a "profile"). SEED index 5 (TartarusX) is
// deliberately LEFT OUT so the preview shows the graceful fallback (a plain
// tinted Spartan with no emblem) for a player who has no profile.
export function mockProfiles(): Record<
	string,
	{ ce?: { color: number }; h2?: { appearance: Appearance } }
> {
	const out: Record<string, { ce?: { color: number }; h2?: { appearance: Appearance } }> = {};
	for (const s of SEEDS) {
		if (s.index === 5) continue; // no profile → fallback demo
		out[s.name.toLowerCase()] = {
			ce: { color: s.armorColor },
			h2: { appearance: mockAppearance(s.index) }
		};
	}
	return out;
}
