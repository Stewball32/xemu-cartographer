import { describe, it, expect } from 'vitest';
import {
	buildScoreboard,
	statusStrip,
	teamMeta,
	prettify,
	matchClock,
	mmss,
	buildKillFeed,
	TICKS_PER_SECOND
} from './overlay-view';
import type {
	AnyEvent,
	DeathEvent,
	GamePayload,
	PlayerRef,
	TickPayloadV2,
	ScenarioPayload
} from '$lib/types/scraper-v2';

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
		health: 1,
		shields: 1,
		max_health: 75,
		max_shields: 75,
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

	it('passes through the wire 0..1 health/shields fraction', () => {
		// Wire health/shields are the engine's 0..1 body_vitality fraction
		// (RUNTIME-VERIFIED) — the builder must NOT re-divide by an engine ceiling.
		const vm = buildScoreboard(
			game({
				config: { gametype: 'slayer', is_team_game: false, score_limit: 25, time_limit_ticks: 0 },
				players: [rp({ index: 0, name: 'Alpha' })]
			}),
			tick([tp({ index: 0, health: 0.5, shields: 0, alive: false, respawn_in_ticks: 90 })])
		);
		expect(vm.hasTick).toBe(true);
		const p = vm.players[0];
		expect(p.health).toBeCloseTo(0.5);
		expect(p.shields).toBe(0);
		expect(p.alive).toBe(false);
		expect(p.respawnIn).toBe(90);
	});

	it('clamps over-full shields (overshield reads >1) to 1', () => {
		const vm = buildScoreboard(
			game({ players: [rp({ index: 0, name: 'A' })] }),
			tick([tp({ index: 0, shields: 3 })])
		);
		expect(vm.players[0].shields).toBe(1);
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

describe('mmss', () => {
	it('formats seconds as M:SS with padded seconds', () => {
		expect(mmss(0)).toBe('0:00');
		expect(mmss(9)).toBe('0:09');
		expect(mmss(75)).toBe('1:15');
		expect(mmss(482)).toBe('8:02');
		expect(mmss(600)).toBe('10:00');
	});
	it('floors fractional seconds and clamps negatives to 0:00', () => {
		expect(mmss(59.9)).toBe('0:59');
		expect(mmss(-5)).toBe('0:00');
		expect(mmss(Number.NaN)).toBe('0:00');
	});
});

describe('matchClock', () => {
	it('degrades to an idle 0:00 with no game', () => {
		const vm = matchClock(null);
		expect(vm.phase).toBe('idle');
		expect(vm.live).toBe(false);
		expect(vm.label).toBe('0:00');
		expect(vm.direction).toBe('up');
		expect(vm.remainingSeconds).toBeNull();
		expect(vm.hasGame).toBe(false);
	});

	it('counts up from engine_tick when there is no time limit', () => {
		// 14400 ticks ÷ 30 Hz = 480s = 8:00.
		const vm = matchClock(
			game({
				phase: 'live',
				engine_tick: 14400,
				config: {
					gametype: 'slayer',
					variant_name: 'Team Slayer',
					is_team_game: true,
					score_limit: 50,
					time_limit_ticks: 0
				}
			})
		);
		expect(vm.direction).toBe('up');
		expect(vm.elapsedSeconds).toBe(480);
		expect(vm.label).toBe('8:00');
		expect(vm.live).toBe(true);
		expect(vm.gametype).toBe('Team Slayer');
		expect(vm.scoreLimit).toBe(50);
		expect(vm.remainingSeconds).toBeNull();
	});

	it('counts down toward a time limit when one is set', () => {
		// limit 18000 (10:00), elapsed 14400 (8:00) → 3600 ticks = 2:00 remaining.
		const vm = matchClock(
			game({
				phase: 'live',
				engine_tick: 14400,
				config: { gametype: 'koth', is_team_game: false, score_limit: 0, time_limit_ticks: 18000 }
			})
		);
		expect(vm.direction).toBe('down');
		expect(vm.remainingSeconds).toBe(120);
		expect(vm.label).toBe('2:00');
	});

	it('clamps a past-limit countdown to 0:00 rather than going negative', () => {
		const vm = matchClock(
			game({
				phase: 'live',
				engine_tick: 20000,
				config: { gametype: 'koth', is_team_game: false, score_limit: 0, time_limit_ticks: 18000 }
			})
		);
		expect(vm.remainingSeconds).toBe(0);
		expect(vm.label).toBe('0:00');
	});

	it('exposes the documented 30 Hz tick rate', () => {
		expect(TICKS_PER_SECOND).toBe(30);
	});
});

function pref(over: Partial<PlayerRef>): PlayerRef {
	return { index: 0, name: '', team: 0, armor_color: 0, ...over };
}

function death(over: Partial<DeathEvent>): DeathEvent {
	return {
		seq: 1,
		tick: 100,
		at: '',
		event_type: 'death',
		victim: pref({ index: 1, name: 'Victim', team: 1 }),
		victim_pos: { x: 0, y: 0, z: 0 },
		killer: pref({ index: 0, name: 'Killer', team: 0 }),
		killer_pos: { x: 0, y: 0, z: 0 },
		cause: 'kill',
		weapon: 'weapons\\pistol\\pistol',
		team_kill: false,
		respawn_in_ticks: 90,
		...over
	};
}

describe('buildKillFeed', () => {
	it('degrades to an empty feed with no events', () => {
		expect(buildKillFeed(null).count).toBe(0);
		expect(buildKillFeed([]).rows).toEqual([]);
	});

	it('ignores non-death events', () => {
		const events: AnyEvent[] = [
			{
				seq: 1,
				tick: 10,
				at: '',
				event_type: 'medal',
				kind: 'multikill',
				player: pref({ name: 'A' }),
				count: 2
			},
			death({ seq: 2, tick: 20 })
		];
		const vm = buildKillFeed(events);
		expect(vm.count).toBe(1);
		expect(vm.rows[0].victimName).toBe('Victim');
	});

	it('maps a normal cross-team kill with killer + victim + weapon', () => {
		const vm = buildKillFeed([death({ seq: 7, tick: 42 })]);
		const r = vm.rows[0];
		expect(r.key).toBe('7-42');
		expect(r.killerName).toBe('Killer');
		expect(r.killerColor).toBe(teamMeta(0).color);
		expect(r.victimName).toBe('Victim');
		expect(r.victimColor).toBe(teamMeta(1).color);
		expect(r.weapon).toBe('Pistol');
		expect(r.selfInflicted).toBe(false);
		expect(r.teamKill).toBe(false);
	});

	it('treats a null killer (fall) as self-inflicted', () => {
		const vm = buildKillFeed([
			death({ killer: null, killer_pos: null, cause: 'fall', weapon: '' })
		]);
		const r = vm.rows[0];
		expect(r.selfInflicted).toBe(true);
		expect(r.killerName).toBeNull();
		expect(r.weapon).toBe('');
	});

	it('treats a suicide as self-inflicted even when a killer ref is present', () => {
		const vm = buildKillFeed([death({ cause: 'suicide' })]);
		expect(vm.rows[0].selfInflicted).toBe(true);
	});

	it('flags a betrayal as a team kill', () => {
		const vm = buildKillFeed([death({ cause: 'betrayal', team_kill: true })]);
		expect(vm.rows[0].teamKill).toBe(true);
		expect(vm.rows[0].selfInflicted).toBe(false);
	});

	it('caps to the requested max, preserving newest-first input order', () => {
		const events = [5, 4, 3, 2, 1].map((n) => death({ seq: n, tick: n }));
		const vm = buildKillFeed(events, 3);
		expect(vm.count).toBe(3);
		expect(vm.rows.map((r) => r.key)).toEqual(['5-5', '4-4', '3-3']);
	});
});
