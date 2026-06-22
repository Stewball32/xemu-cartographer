import { describe, it, expect } from 'vitest';
import {
	buildVizModel,
	classifyTag,
	computeBounds,
	healthColor,
	makeProjection
} from './visualizer-view';
import type {
	GamePayload,
	ObjectsPayload,
	ScenarioPayload,
	TickPayloadV2
} from '$lib/types/scraper-v2';

function vec(x: number, y: number, z = 0) {
	return { x, y, z };
}

describe('computeBounds', () => {
	it('returns invalid for no points', () => {
		expect(computeBounds([], 'static').valid).toBe(false);
		expect(computeBounds([], 'static').source).toBe('none');
	});

	it('fits min/max over x and y and tracks z range', () => {
		const b = computeBounds([vec(-10, 5, 1), vec(40, -20, 9), vec(0, 0, 3)], 'static');
		expect(b.valid).toBe(true);
		expect(b.minX).toBe(-10);
		expect(b.maxX).toBe(40);
		expect(b.minY).toBe(-20);
		expect(b.maxY).toBe(5);
		expect(b.minZ).toBe(1);
		expect(b.maxZ).toBe(9);
		expect(b.source).toBe('static');
	});

	it('enforces a minimum span around clustered points (no zero-span)', () => {
		const b = computeBounds([vec(100, 100), vec(101, 100.5)], 'dynamic');
		expect(b.maxX - b.minX).toBeGreaterThanOrEqual(10);
		expect(b.maxY - b.minY).toBeGreaterThanOrEqual(10);
		// centred on the cluster
		expect((b.minX + b.maxX) / 2).toBeCloseTo(100.5, 5);
	});

	it('ignores non-finite points', () => {
		const b = computeBounds([vec(0, 0), { x: NaN, y: 5, z: 0 }, vec(20, 20)], 'static');
		expect(b.maxX).toBe(20);
		expect(b.maxY).toBe(20);
	});
});

describe('makeProjection', () => {
	const bounds = computeBounds([vec(-50, -50), vec(50, 50)], 'static');
	const proj = makeProjection(bounds, 1000, 1000, 0);

	it('maps the world centre to the viewport centre', () => {
		const p = proj.project(vec(0, 0));
		expect(p.x).toBeCloseTo(500, 5);
		expect(p.y).toBeCloseTo(500, 5);
	});

	it('flips Y: world +Y maps to the top of the viewport', () => {
		const top = proj.project(vec(0, 50)); // max Y
		const bot = proj.project(vec(0, -50)); // min Y
		expect(top.y).toBeLessThan(bot.y);
		expect(top.y).toBeCloseTo(0, 5);
		expect(bot.y).toBeCloseTo(1000, 5);
	});

	it('world +X maps right', () => {
		const left = proj.project(vec(-50, 0));
		const right = proj.project(vec(50, 0));
		expect(right.x).toBeGreaterThan(left.x);
	});

	it('is aspect-preserving and centres a non-square box', () => {
		// wide box (200 x 100) into a square viewport → letterboxed vertically
		const wide = computeBounds([vec(-100, -50), vec(100, 50)], 'static');
		const p = makeProjection(wide, 1000, 1000, 0);
		expect(p.scale).toBeCloseTo(1000 / 200, 5);
		const centre = p.project(vec(0, 0));
		expect(centre.x).toBeCloseTo(500, 5);
		expect(centre.y).toBeCloseTo(500, 5);
	});
});

describe('classifyTag', () => {
	it('classifies by tag path head', () => {
		expect(classifyTag('vehicles\\warthog\\warthog')).toBe('vehicle');
		expect(classifyTag('weapons\\rocket launcher\\rocket launcher')).toBe('weapon');
		expect(classifyTag('powerups\\over shield\\over shield')).toBe('powerup');
		expect(classifyTag('characters\\cyborg\\cyborg')).toBe('biped');
	});

	it('falls back to the numeric type when the tag is unhelpful', () => {
		expect(classifyTag('', 1)).toBe('vehicle');
		expect(classifyTag(undefined, 2)).toBe('weapon');
		expect(classifyTag('mystery', undefined)).toBe('other');
	});
});

describe('healthColor', () => {
	it('ramps green (full) → red (empty)', () => {
		expect(healthColor(1)).toContain('rgb');
		// full is greener than empty
		const full = healthColor(1);
		const empty = healthColor(0);
		expect(full).not.toBe(empty);
	});
});

describe('buildVizModel', () => {
	const game: GamePayload = {
		phase: 'live',
		started_at: '',
		last_read_at: '',
		engine_tick: 0,
		iterations: 1,
		config: {
			gametype: 'ctf',
			variant_name: 'Capture the Flag',
			is_team_game: true,
			score_limit: 3,
			time_limit_ticks: 0
		},
		team_scores: [],
		players: [
			{
				index: 0,
				name: 'Red1',
				team: 0,
				armor_color: 0,
				score: 0,
				kills: 0,
				deaths: 0,
				assists: 0,
				ctf_score: 0,
				team_kills: 0,
				suicides: 0,
				kill_streak: 0,
				multikill: 0,
				shots_fired: 0,
				shots_hit: 0,
				is_local: true,
				local_index: 0,
				machine_index: 0,
				controller_index: 0
			},
			{
				index: 1,
				name: 'Blue1',
				team: 1,
				armor_color: 1,
				score: 0,
				kills: 0,
				deaths: 0,
				assists: 0,
				ctf_score: 0,
				team_kills: 0,
				suicides: 0,
				kill_streak: 0,
				multikill: 0,
				shots_fired: 0,
				shots_hit: 0,
				is_local: false,
				local_index: null,
				machine_index: 1,
				controller_index: null
			}
		],
		machines: [],
		network: null
	};

	const tick: TickPayloadV2 = {
		players: [
			{
				index: 0,
				alive: true,
				respawn_in_ticks: null,
				pos: vec(-40, 40, 0),
				vel: vec(0, 0, 0),
				aim: vec(1, 0, 0),
				zoom_level: 0,
				crouch_scale: 0,
				health: 1,
				shields: 1,
				max_health: 75,
				max_shields: 75,
				has_camo: false,
				has_overshield: false,
				frags: 2,
				plasmas: 0,
				selected_weapon_slot: 0,
				biped_tag: 'characters\\cyborg\\cyborg',
				actions: {
					crouching: false,
					jumping: false,
					firing: false,
					flashlight: false,
					throwing_grenade: false,
					meleeing: false,
					using: false
				},
				weapons: []
			},
			{
				index: 1,
				alive: false,
				respawn_in_ticks: 60,
				pos: vec(40, -40, 0),
				vel: vec(0, 0, 0),
				aim: vec(0, 0, 0),
				zoom_level: 0,
				crouch_scale: 0,
				health: 0,
				shields: 0,
				max_health: 75,
				max_shields: 75,
				has_camo: false,
				has_overshield: false,
				frags: 0,
				plasmas: 0,
				selected_weapon_slot: 0,
				biped_tag: 'characters\\cyborg\\cyborg',
				actions: {
					crouching: false,
					jumping: false,
					firing: false,
					flashlight: false,
					throwing_grenade: false,
					meleeing: false,
					using: false
				},
				weapons: []
			}
		],
		power_items: [
			{ spawn_id: 0, status: 'world', held_by: null, pos: vec(0, 0, 0), respawn_in_ticks: null },
			{ spawn_id: 1, status: 'held', held_by: 0, pos: vec(-40, 40, 0), respawn_in_ticks: null },
			{ spawn_id: 2, status: 'respawning', held_by: null, pos: null, respawn_in_ticks: 300 }
		],
		ctf_flags: [{ team: 0, pos: vec(-50, 50, 0), carrier: null, status: 'home' }],
		game_globals: null,
		locals: []
	};

	const scenario: ScenarioPayload = {
		map: 'levels\\test\\bloodgulch\\bloodgulch',
		game_difficulty: 2,
		fog: null,
		memory_regions: null,
		object_types: [],
		player_spawns: [
			{ index: 0, pos: vec(-55, 55, 0), facing: 0, team_index: 0, bsp_index: 0, gametypes: [] },
			{ index: 1, pos: vec(55, -55, 0), facing: 0, team_index: 1, bsp_index: 0, gametypes: [] }
		],
		power_item_spawns: [
			{
				spawn_id: 0,
				tag: 'weapons\\rocket launcher\\rocket launcher',
				interval_ticks: 3600,
				gametype_mask: 0,
				pos: vec(0, 0, 0)
			}
		],
		tag_defs: {}
	};

	const objects: ObjectsPayload = {
		objects: [
			{
				object_id: 100,
				tag: 'vehicles\\warthog\\warthog',
				type: 1,
				flags: 0,
				pos: vec(-45, 40, 0),
				ang_vel: vec(0, 0, 0),
				time_existing: 10,
				owner_unit: null,
				owner_object: null,
				ultimate_parent: 100
			},
			{
				object_id: 101,
				tag: 'vehicles\\ghost\\ghost',
				type: 1,
				flags: 0,
				pos: vec(45, -40, 0),
				ang_vel: vec(0, 0, 0),
				time_existing: 10,
				owner_unit: 999,
				owner_object: null,
				ultimate_parent: 101
			},
			{
				object_id: 102,
				tag: 'weapons\\plasma rifle\\plasma rifle',
				type: 2,
				flags: 0,
				pos: vec(10, 5, 0),
				ang_vel: vec(0, 0, 0),
				time_existing: 5,
				owner_unit: null,
				owner_object: null,
				ultimate_parent: 102
			}
		],
		projectiles: [
			{
				object_id: 200,
				tag: 'weapons\\frag grenade\\frag grenade',
				pos: vec(5, -5, 1),
				flags: 0,
				action: 0,
				detonation_timer: 1.5,
				distance_traveled: 3
			}
		]
	};

	it('joins roster identity with tick positions', () => {
		const m = buildVizModel(game, tick, scenario, objects);
		expect(m.players).toHaveLength(2);
		const red = m.players.find((p) => p.index === 0)!;
		expect(red.name).toBe('Red1');
		expect(red.team).toBe(0);
		expect(red.isLocal).toBe(true);
		expect(red.pos.x).toBe(-40);
		expect(red.heading).toBeCloseTo(0, 5); // aim +x → screen 0
		const blue = m.players.find((p) => p.index === 1)!;
		expect(blue.alive).toBe(false);
		expect(blue.heading).toBeNull(); // zero aim
	});

	it('labels power items from scenario spawns and counts respawning', () => {
		const m = buildVizModel(game, tick, scenario, objects);
		const rocket = m.items.find((i) => i.id === 'item-0')!;
		expect(rocket.label).toBe('Rocket Launcher');
		expect(rocket.kind).toBe('weapon');
		// held item attaches to a player, not a world dot
		expect(m.items.find((i) => i.id === 'item-1')!.heldBy).toBe(0);
		// respawning item is counted, not placed
		expect(m.respawningItems).toBe(1);
	});

	it('splits objects into vehicles + dropped world items', () => {
		const m = buildVizModel(game, tick, scenario, objects);
		expect(m.vehicles).toHaveLength(2);
		expect(m.vehicles.find((v) => v.id === 'veh-101')!.occupied).toBe(true);
		expect(m.vehicles.find((v) => v.id === 'veh-100')!.occupied).toBe(false);
		// dropped plasma rifle (no owner) surfaces as an item
		expect(m.items.some((i) => i.id === 'obj-102')).toBe(true);
		expect(m.projectiles).toHaveLength(1);
	});

	it('reads CTF flags and static spawns', () => {
		const m = buildVizModel(game, tick, scenario, objects);
		expect(m.flags).toHaveLength(1);
		expect(m.flags[0].status).toBe('home');
		expect(m.spawns).toHaveLength(2);
	});

	it('prefers static bounds (stable) and unions live entities', () => {
		const m = buildVizModel(game, tick, scenario, objects);
		expect(m.bounds.valid).toBe(true);
		expect(m.bounds.source).toBe('static');
		// spawns at ±55 dominate the box
		expect(m.bounds.minX).toBeLessThanOrEqual(-55);
		expect(m.bounds.maxX).toBeGreaterThanOrEqual(55);
	});

	it('sets capability flags and counts', () => {
		const m = buildVizModel(game, tick, scenario, objects);
		expect(m.hasGame && m.hasTick && m.hasScenario && m.hasObjects).toBe(true);
		expect(m.mapName).toBe('Bloodgulch');
		expect(m.gametype).toBe('Capture The Flag');
		expect(m.playerCount).toBe(2);
		expect(m.placedCount).toBe(2);
	});

	it('degrades cleanly with only a tick (no scenario/objects)', () => {
		const m = buildVizModel(game, tick, null, null);
		expect(m.hasScenario).toBe(false);
		expect(m.hasObjects).toBe(false);
		expect(m.vehicles).toHaveLength(0);
		expect(m.spawns).toHaveLength(0);
		// bounds fall back to dynamic (live entity) positions
		expect(m.bounds.valid).toBe(true);
		expect(m.bounds.source).toBe('dynamic');
	});

	it('returns an empty, invalid-bounds model with no payloads', () => {
		const m = buildVizModel(null, null, null, null);
		expect(m.players).toHaveLength(0);
		expect(m.bounds.valid).toBe(false);
		expect(m.hasPositions).toBe(false);
		expect(m.playerCount).toBe(0);
	});
});
