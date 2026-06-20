import { describe, it, expect } from 'vitest';
import {
	buildScoreboard,
	statusStrip,
	teamMeta,
	prettify,
	HEALTH_MAX,
	SHIELD_MAX
} from './overlay-view';
import type { GamePayload, TickPayloadV2, ScenarioPayload } from '$lib/types/scraper-v2';

// Minimal roster-player factory — only the fields the builders read need to be
// realistic; the rest carry zero defaults.
function rp(over: Partial<GamePayload['players'][number]>): GamePayload['players'][number] {
	return {
		index: 0,
		name: '',
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
		is_local: null,
		local_index: null,
		machine_index: null,
		controller_index: null,
		...over
	};
}

function tp(over: Partial<TickPayloadV2['players'][number]>): TickPayloadV2['players'][number] {
	return {
		index: 0,
		alive: true,
		respawn_in_ticks: null,
		pos: { x: 0, y: 0, z: 0 },
		vel: { x: 0, y: 0, z: 0 },
		aim: { x: 0, y: 0, z: 0 },
		zoom_level: 0,
		crouch_scale: 0,
		health: HEALTH_MAX,
		shields: SHIELD_MAX,
		has_camo: false,
		has_overshield: false,
		frags: 0,
		plasmas: 0,
		selected_weapon_slot: 0,
		biped_tag: '',
		actions: {
			crouching: false,
			jumping: false,
			firing: false,
			flashlight: false,
			throwing_grenade: false,
			meleeing: false,
			using: false
		},
		weapons: [],
		...over
	};
}

function game(over: Partial<GamePayload>): GamePayload {
	return {
		phase: 'live',
		started_at: '',
		last_read_at: '',
		engine_tick: 0,
		iterations: 0,
		config: null,
		team_scores: [],
		players: [],
		machines: [],
		network: null,
		...over
	};
}

function tick(players: TickPayloadV2['players']): TickPayloadV2 {
	return { players, power_items: [], ctf_flags: [], game_globals: null, locals: [] };
}

describe('prettify', () => {
	it('strips path segments and title-cases', () => {
		expect(prettify('levels\\test\\bloodgulch\\bloodgulch')).toBe('Bloodgulch');
		expect(prettify('team_slayer')).toBe('Team Slayer');
		expect(prettify('capture-the-flag')).toBe('Capture The Flag');
	});
	it('degrades empty / nullish to empty string', () => {
		expect(prettify('')).toBe('');
		expect(prettify(null)).toBe('');
		expect(prettify(undefined)).toBe('');
	});
});

describe('teamMeta', () => {
	it('maps the four primary teams', () => {
		expect(teamMeta(0).name).toBe('Red');
		expect(teamMeta(1).name).toBe('Blue');
		expect(teamMeta(2).name).toBe('Green');
		expect(teamMeta(3).name).toBe('Gold');
	});
	it('wraps out-of-range indices defensively', () => {
		expect(teamMeta(8).name).toBe('Red');
		expect(teamMeta(-1).name).toBe('Pink');
	});
});

describe('buildScoreboard', () => {
	it('returns an empty idle VM with no game', () => {
		const vm = buildScoreboard(null, null);
		expect(vm.playerCount).toBe(0);
		expect(vm.teams).toEqual([]);
		expect(vm.players).toEqual([]);
		expect(vm.hasTick).toBe(false);
		expect(vm.hasScores).toBe(false);
	});

	it('renders a roster from game alone (no tick) with hasTick false', () => {
		const vm = buildScoreboard(
			game({
				config: { gametype: 'slayer', is_team_game: false, score_limit: 25, time_limit_ticks: 0 },
				players: [rp({ index: 0, name: 'Alpha' }), rp({ index: 1, name: 'Bravo' })]
			}),
			null
		);
		expect(vm.hasTick).toBe(false);
		expect(vm.isTeamGame).toBe(false);
		expect(vm.players.map((p) => p.name)).toEqual(['Alpha', 'Bravo']);
	});

	it('skips empty-name roster slots', () => {
		const vm = buildScoreboard(
			game({ players: [rp({ index: 0, name: 'Alpha' }), rp({ index: 1, name: '   ' })] }),
			null
		);
		expect(vm.playerCount).toBe(1);
		expect(vm.players[0].name).toBe('Alpha');
	});

	it('groups team games by team and orders teams by index', () => {
		const vm = buildScoreboard(
			game({
				config: { gametype: 'slayer', is_team_game: true, score_limit: 50, time_limit_ticks: 0 },
				team_scores: [
					{ team: 1, score: 30 },
					{ team: 0, score: 42 }
				],
				players: [
					rp({ index: 0, name: 'RedOne', team: 0, kills: 10 }),
					rp({ index: 1, name: 'BlueOne', team: 1, kills: 5 })
				]
			}),
			null
		);
		expect(vm.isTeamGame).toBe(true);
		expect(vm.teams.map((t) => t.team)).toEqual([0, 1]);
		expect(vm.teams[0].name).toBe('Red');
		expect(vm.teams[0].score).toBe(42); // from team_scores, not summed
		expect(vm.teams[1].score).toBe(30);
		expect(vm.hasScores).toBe(true);
	});

	it('falls back to summed member score when team_scores absent', () => {
		const vm = buildScoreboard(
			game({
				config: { gametype: 'slayer', is_team_game: true, score_limit: 50, time_limit_ticks: 0 },
				players: [
					rp({ index: 0, name: 'A', team: 0, score: 7 }),
					rp({ index: 1, name: 'B', team: 0, score: 3 })
				]
			}),
			null
		);
		expect(vm.teams[0].score).toBe(10);
	});

	it('infers a team game from distinct team indices when config is null', () => {
		const vm = buildScoreboard(
			game({
				config: null,
				players: [rp({ index: 0, name: 'A', team: 0 }), rp({ index: 1, name: 'B', team: 1 })]
			}),
			null
		);
		expect(vm.isTeamGame).toBe(true);
	});

	it('joins tick state and normalises health/shields to 0..1', () => {
		const vm = buildScoreboard(
			game({
				config: { gametype: 'slayer', is_team_game: false, score_limit: 25, time_limit_ticks: 0 },
				players: [rp({ index: 0, name: 'Alpha' })]
			}),
			tick([
				tp({ index: 0, health: HEALTH_MAX / 2, shields: 0, alive: false, respawn_in_ticks: 90 })
			])
		);
		expect(vm.hasTick).toBe(true);
		const p = vm.players[0];
		expect(p.health).toBeCloseTo(0.5);
		expect(p.shields).toBe(0);
		expect(p.alive).toBe(false);
		expect(p.respawnIn).toBe(90);
	});

	it('clamps over-max health to 1', () => {
		const vm = buildScoreboard(
			game({ players: [rp({ index: 0, name: 'A' })] }),
			tick([tp({ index: 0, health: HEALTH_MAX * 3 })])
		);
		expect(vm.players[0].health).toBe(1);
	});

	it('reports hasScores false when every stat is zero (unverified/absent)', () => {
		const vm = buildScoreboard(
			game({
				config: { gametype: 'slayer', is_team_game: false, score_limit: 25, time_limit_ticks: 0 },
				players: [rp({ index: 0, name: 'A' }), rp({ index: 1, name: 'B' })]
			}),
			null
		);
		expect(vm.hasScores).toBe(false);
	});

	it('orders FFA players by score then kills then name', () => {
		const vm = buildScoreboard(
			game({
				config: { gametype: 'slayer', is_team_game: false, score_limit: 25, time_limit_ticks: 0 },
				players: [
					rp({ index: 0, name: 'Low', score: 1 }),
					rp({ index: 1, name: 'High', score: 9 }),
					rp({ index: 2, name: 'Mid', score: 5 })
				]
			}),
			null
		);
		expect(vm.players.map((p) => p.name)).toEqual(['High', 'Mid', 'Low']);
	});
});

describe('statusStrip', () => {
	it('degrades to an idle strip with no game', () => {
		const vm = statusStrip(null, null);
		expect(vm.phase).toBe('idle');
		expect(vm.teams).toEqual([]);
		expect(vm.hasScores).toBe(false);
		expect(vm.map).toBe('');
	});

	it('projects gametype, variant, map, and team scores', () => {
		const vm = statusStrip(
			game({
				config: {
					gametype: 'ctf',
					variant_name: 'Capture the Flag',
					is_team_game: true,
					score_limit: 3,
					time_limit_ticks: 0
				},
				team_scores: [
					{ team: 0, score: 2 },
					{ team: 1, score: 1 }
				]
			}),
			{ map: 'bloodgulch' } as ScenarioPayload
		);
		expect(vm.gametype).toBe('Ctf');
		expect(vm.variant).toBe('Capture the Flag');
		expect(vm.map).toBe('Bloodgulch');
		expect(vm.isTeamGame).toBe(true);
		expect(vm.scoreLimit).toBe(3);
		expect(vm.teams.map((t) => `${t.name}:${t.score}`)).toEqual(['Red:2', 'Blue:1']);
		expect(vm.hasScores).toBe(true);
	});

	it('marks hasScores false when scores are all zero', () => {
		const vm = statusStrip(
			game({
				config: { gametype: 'slayer', is_team_game: true, score_limit: 50, time_limit_ticks: 0 },
				team_scores: [
					{ team: 0, score: 0 },
					{ team: 1, score: 0 }
				]
			}),
			null
		);
		expect(vm.hasScores).toBe(false);
	});
});
