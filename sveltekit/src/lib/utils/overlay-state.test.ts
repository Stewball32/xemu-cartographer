import { describe, it, expect } from 'vitest';
import { overlayPlayers } from './overlay-state';
import type { GamePayload, TickPayloadV2 } from '$lib/types/scraper-v2';

// Mirrors the live beta-stream lobby: three consoles, each with one player, on
// shifting machine indices. Per-console (POV) selection MUST pick the seat by
// its resolved machine index so BlueBox shows BlueBox's player, never RedBox's.
function fixture(): { game: GamePayload; tick: TickPayloadV2 } {
	const p = (index: number, name: string, team: number, machine_index: number) =>
		({
			index,
			name,
			team,
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
			is_local: machine_index === 0,
			local_index: machine_index === 0 ? 0 : null,
			machine_index,
			controller_index: 0
		}) as GamePayload['players'][number];
	return {
		game: {
			phase: 'live',
			started_at: '',
			last_read_at: '',
			engine_tick: 0,
			iterations: 0,
			config: { gametype: 'slayer', is_team_game: false, score_limit: 0, time_limit_ticks: 0 },
			team_scores: [],
			// machine 0 stream(Crazy), 1 RedBox(OG50 II), 2 BlueBox(Stewball32)
			players: [p(0, 'Crazy', 0, 0), p(1, 'OG50 II', 0, 1), p(2, 'Stewball32', 1, 2)],
			machines: [
				{ index: 0, name: 'stream', is_local: true },
				{ index: 1, name: 'RedBox', is_local: false },
				{ index: 2, name: 'BlueBox', is_local: false }
			],
			network: null
		} as GamePayload,
		tick: { players: [], power_items: [], ctf_flags: [], game_globals: null, locals: [] }
	};
}

describe('overlayPlayers machine filter (per-console POV)', () => {
	const { game, tick } = fixture();

	it('whole roster when no machine index', () => {
		expect(overlayPlayers(game, tick).map((p) => p.name)).toEqual([
			'Crazy',
			'OG50 II',
			'Stewball32'
		]);
	});

	it('BlueBox (machine 2) → only Stewball32, distinct from RedBox', () => {
		expect(overlayPlayers(game, tick, 2).map((p) => p.name)).toEqual(['Stewball32']);
	});

	it('RedBox (machine 1) → only OG50 II', () => {
		expect(overlayPlayers(game, tick, 1).map((p) => p.name)).toEqual(['OG50 II']);
	});

	it('maps betrayals (team_kills) and suicides straight through', () => {
		const g = fixture().game;
		g.players[1].team_kills = 2;
		g.players[1].suicides = 4;
		const [og] = overlayPlayers(g, tick, 1);
		expect(og.betrayals).toBe(2);
		expect(og.suicides).toBe(4);
	});

	it('maps the accumulated match stats (acc_* wire fields)', () => {
		const g = fixture().game;
		Object.assign(g.players[1], {
			acc_shots_fired: 120,
			acc_grenade_throws: 6,
			acc_melees: 3,
			acc_damage_dealt: 1450.5,
			acc_damage_received: 900,
			acc_camo_pickups: 2,
			acc_overshield_pickups: 1,
			best_kill_streak: 7,
			kill_streak: 2
		});
		const [og] = overlayPlayers(g, tick, 1);
		expect(og.shotsFired).toBe(120);
		expect(og.grenadeThrows).toBe(6);
		expect(og.meleeKills).toBe(3);
		expect(og.damageDealt).toBe(1450.5);
		expect(og.damageTaken).toBe(900);
		expect(og.camoPickups).toBe(2);
		expect(og.osPickups).toBe(1);
		expect(og.bestSpree).toBe(7); // peak, distinct from...
		expect(og.spree).toBe(2); // ...the current streak
	});

	it('defaults acc stats to 0 when absent (older server)', () => {
		const [og] = overlayPlayers(fixture().game, tick, 1);
		expect(og.shotsFired).toBe(0);
		expect(og.bestSpree).toBe(0);
		expect(og.damageDealt).toBe(0);
	});

	it('BlueBox and RedBox never resolve to the same player', () => {
		const blue = overlayPlayers(game, tick, 2).map((p) => p.name);
		const red = overlayPlayers(game, tick, 1).map((p) => p.name);
		expect(blue).not.toEqual(red);
	});

	it('machine -1 (own console, no lobby) → whole roster', () => {
		expect(overlayPlayers(game, tick, -1)).toHaveLength(3);
	});
});
