import { describe, it, expect } from 'vitest';
import type { GamePayload, TickPayloadV2 } from '$lib/types/scraper-v2';
import { deriveSplitCount, layoutKey, localOverlayPlayers } from './overlay-split';

// Minimal roster/tick fixtures — only the fields the split logic reads.
function roster(...specs: Array<Partial<Record<string, unknown>>>) {
	return specs.map((s, i) => ({
		index: i,
		name: `P${i}`,
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
		...s
	}));
}

function game(players: unknown[], isTeamGame = false): GamePayload {
	return { config: { is_team_game: isTeamGame }, players } as unknown as GamePayload;
}

describe('deriveSplitCount', () => {
	it('counts local players (splitscreen locals), ignoring remotes', () => {
		const g = game(
			roster(
				{ is_local: true, local_index: 0 },
				{ is_local: true, local_index: 1 },
				{ is_local: false, local_index: null }, // remote/system-link
				{ is_local: false, local_index: null }
			)
		);
		expect(deriveSplitCount(g, null)).toBe(2);
	});

	it('is 1 for a single local player', () => {
		expect(deriveSplitCount(game(roster({ is_local: true, local_index: 0 })), null)).toBe(1);
	});

	it('caps at the engine 4-way max', () => {
		const locals = roster(
			{ is_local: true, local_index: 0 },
			{ is_local: true, local_index: 1 },
			{ is_local: true, local_index: 2 },
			{ is_local: true, local_index: 3 }
		);
		expect(deriveSplitCount(game(locals), null)).toBe(4);
	});

	it('falls back to tick.locals when the roster has no is_local yet', () => {
		const tick = {
			players: [],
			locals: [{ local_index: 0 }, { local_index: 1 }]
		} as unknown as TickPayloadV2;
		expect(deriveSplitCount(game([]), tick)).toBe(2);
	});

	it('is 0 with no local game', () => {
		expect(deriveSplitCount(null, null)).toBe(0);
		expect(deriveSplitCount(game(roster({ is_local: false })), null)).toBe(0);
	});
});

describe('layoutKey', () => {
	it('maps 0 → 1 (idle single view) and clamps 1..4', () => {
		expect(layoutKey(0)).toBe(1);
		expect(layoutKey(1)).toBe(1);
		expect(layoutKey(2)).toBe(2);
		expect(layoutKey(4)).toBe(4);
		expect(layoutKey(9)).toBe(4);
	});
});

describe('localOverlayPlayers', () => {
	it('maps locals ordered by local_index and joins tick health/alive', () => {
		const g = game(
			roster(
				{
					index: 1,
					is_local: true,
					local_index: 1,
					name: 'BOT',
					team: 1,
					kills: 5,
					deaths: 2,
					kill_streak: 3
				},
				{
					index: 0,
					is_local: true,
					local_index: 0,
					name: 'STEW',
					team: 0,
					shots_fired: 10,
					shots_hit: 5
				}
			),
			true // team game
		);
		const tick = {
			players: [
				{ index: 0, alive: true, health: 0.5, shields: 1, respawn_in_ticks: null, has_camo: false },
				{ index: 1, alive: false, health: 0, shields: 0, respawn_in_ticks: 90, has_camo: true }
			],
			locals: []
		} as unknown as TickPayloadV2;

		const out = localOverlayPlayers(g, tick);
		expect(out.map((p) => p.name)).toEqual(['STEW', 'BOT']); // sorted by local_index 0,1
		expect(out[0].slot).toBe(0);
		expect(out[0].team).toBe('red'); // team 0, team game
		expect(out[0].accuracy).toBeCloseTo(50);
		expect(out[0].health).toBe(0.5);
		expect(out[1].team).toBe('blue'); // team 1
		expect(out[1].alive).toBe(false);
		expect(out[1].spree).toBe(3);
		expect(out[1].respawn).toBe(3); // 90 ticks / 30 = 3s
		expect(out[1].camo).toBe(1); // boolean contract — PlayerCard reads >1 as a percent
	});

	it('keeps respawn seconds fractional so the ring arc drains', () => {
		// 90 ticks divides evenly, which is why the rounded and unrounded
		// mappings were indistinguishable above. 91 does not.
		const g = game(roster({ index: 0, is_local: true, local_index: 0, name: 'STEW' }), false);
		const tick = {
			players: [
				{ index: 0, alive: false, health: 0, shields: 0, respawn_in_ticks: 91, has_camo: false }
			],
			locals: []
		} as unknown as TickPayloadV2;

		expect(localOverlayPlayers(g, tick)[0].respawn).toBeCloseTo(91 / 30, 5);
	});

	it('is ffa team when not a team game', () => {
		const out = localOverlayPlayers(game(roster({ is_local: true, local_index: 0 }), false), null);
		expect(out[0].team).toBe('ffa');
	});
});
