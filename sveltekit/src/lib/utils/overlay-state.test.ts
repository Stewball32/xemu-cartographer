import { describe, it, expect } from 'vitest';
import {
	applyIdentities,
	compactNumber,
	createClockLatch,
	matchState,
	matchTotals,
	ordinal,
	overlayPlayers,
	rankPlayers
} from './overlay-state';
import { damageRatioOf, type OverlayPlayer } from './overlay-split';
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

// ---------------------------------------------------------------------------
// Match-level mapping + the shared formatters the new graphics pack leans on.
// ---------------------------------------------------------------------------

function player(over: Partial<OverlayPlayer> = {}): OverlayPlayer {
	return {
		slot: 0,
		name: 'P',
		display: 'P',
		avatar: null,
		team: 'ffa',
		armor: '#fff',
		score: 0,
		kills: 0,
		deaths: 0,
		assists: 0,
		spree: 0,
		bestSpree: 0,
		accuracy: 0,
		damageRatio: 0,
		betrayals: 0,
		suicides: 0,
		shotsFired: 0,
		grenadeThrows: 0,
		meleeKills: 0,
		damageDealt: 0,
		damageTaken: 0,
		camoPickups: 0,
		osPickups: 0,
		alive: true,
		respawn: 0,
		respawnMax: 5,
		camo: 0,
		camoMax: 30,
		shield: 1,
		health: 1,
		...over
	};
}

function gameWith(over: Partial<GamePayload> = {}): GamePayload {
	return {
		...fixture().game,
		...over
	};
}

describe('matchState', () => {
	it('FFA → mode ffa and teams null', () => {
		const m = matchState(gameWith(), null);
		expect(m.mode).toBe('ffa');
		expect(m.teams).toBeNull();
	});

	it('team game → mode team, both teams named and scored', () => {
		const m = matchState(
			gameWith({
				config: { gametype: 'slayer', is_team_game: true, score_limit: 50, time_limit_ticks: 0 },
				team_scores: [
					{ team: 0, score: 31 },
					{ team: 1, score: 27 }
				]
			}),
			null
		);
		expect(m.mode).toBe('team');
		expect(m.teams).toEqual([
			{ id: 'red', name: 'RED TEAM', score: 31 },
			{ id: 'blue', name: 'BLUE TEAM', score: 27 }
		]);
	});

	it('map name reduces the tag path to an upper-cased basename', () => {
		const m = matchState(gameWith(), {
			map: 'levels\\test\\bloodgulch\\bloodgulch'
		} as never);
		expect(m.map).toBe('BLOODGULCH');
	});

	it('clock counts engine_tick up at 30Hz while live', () => {
		expect(matchState(gameWith({ engine_tick: 30 * 522 }), null).clock).toBe('8:42');
	});

	it('clock is undefined outside a live match (engine_tick free-runs at the menu)', () => {
		expect(
			matchState(gameWith({ phase: 'idle', engine_tick: 999999 }), null).clock
		).toBeUndefined();
	});

	it('rules line names the limit per gametype', () => {
		const rules = (gametype: string, score_limit: number) =>
			matchState(
				gameWith({ config: { gametype, is_team_game: false, score_limit, time_limit_ticks: 0 } }),
				null
			).rules;
		expect(rules('slayer', 15)).toBe('SLAYER — KILL LIMIT 15');
		expect(rules('ctf', 3)).toBe('CTF — FLAG LIMIT 3');
		expect(rules('oddball', 60)).toBe('ODDBALL — TARGET TIME 60');
		expect(rules('slayer', 0)).toBe('SLAYER'); // no limit set → gametype alone
	});
});

describe('rankPlayers', () => {
	it('sorts by score, then kills, then fewest deaths', () => {
		const ranked = rankPlayers([
			player({ name: 'C', score: 5, kills: 5, deaths: 2 }),
			player({ name: 'A', score: 9, kills: 9, deaths: 4 }),
			player({ name: 'B', score: 5, kills: 5, deaths: 1 })
		]);
		expect(ranked.map((p) => p.name)).toEqual(['A', 'B', 'C']);
	});

	it('does not mutate the input array', () => {
		const input = [player({ name: 'X', score: 1 }), player({ name: 'Y', score: 9 })];
		rankPlayers(input);
		expect(input.map((p) => p.name)).toEqual(['X', 'Y']);
	});
});

describe('ordinal', () => {
	it('maps placings and falls back past the lobby cap', () => {
		expect(ordinal(0)).toBe('1ST');
		expect(ordinal(2)).toBe('3RD');
		expect(ordinal(7)).toBe('8TH');
		expect(ordinal(8)).toBe('—');
	});
});

describe('compactNumber', () => {
	it('abbreviates at thousands and millions', () => {
		expect(compactNumber(482)).toBe('482');
		expect(compactNumber(48231)).toBe('48.2K');
		expect(compactNumber(1_250_000)).toBe('1.3M');
	});
});

describe('matchTotals', () => {
	it('sums the lobby and pre-formats damage', () => {
		const totals = matchTotals([
			player({ kills: 12, shotsFired: 800, grenadeThrows: 40, damageDealt: 30000 }),
			player({ kills: 26, shotsFired: 484, grenadeThrows: 56, damageDealt: 18231 })
		]);
		expect(totals).toEqual({ kills: 38, shots: 1284, nades: 96, damage: '48.2K' });
	});

	it('is zero-safe on an empty lobby', () => {
		expect(matchTotals([])).toEqual({ kills: 0, shots: 0, nades: 0, damage: '0' });
	});
});

describe('createClockLatch', () => {
	it('holds the last LIVE tick once the match returns to the menu', () => {
		const latch = createClockLatch();
		latch.observe(gameWith({ phase: 'live', engine_tick: 30 * 60 }));
		latch.observe(gameWith({ phase: 'live', engine_tick: 30 * 686 })); // 11:26
		// Match ends; 0x0C free-runs at the menu and must NOT move the latch.
		latch.observe(gameWith({ phase: 'idle', engine_tick: 30 * 99999 }));
		expect(latch.duration).toBe('11:26');
	});

	it('reports undefined when no live match was ever seen', () => {
		const latch = createClockLatch();
		latch.observe(gameWith({ phase: 'idle', engine_tick: 12345 }));
		expect(latch.duration).toBeUndefined();
	});
});

describe('damageRatio mapping', () => {
	it('derives dealt-per-taken from the accumulator', () => {
		const g = gameWith();
		g.players[0].acc_damage_dealt = 1450;
		g.players[0].acc_damage_received = 1000;
		expect(overlayPlayers(g, null)[0].damageRatio).toBeCloseTo(1.45, 5);
	});

	it('reports an unbounded ratio when no damage was taken', () => {
		const g = gameWith();
		g.players[0].acc_damage_dealt = 900;
		g.players[0].acc_damage_received = 0;
		// The view renders this as ∞. It must NOT be the dealt total — 900 next
		// to a teammate's 1.16 reads as a broken column, not a dominant player.
		expect(overlayPlayers(g, null)[0].damageRatio).toBe(Infinity);
	});
});

describe('damageRatioOf', () => {
	it('is damage dealt divided by damage taken', () => {
		expect(damageRatioOf(1450, 1000)).toBeCloseTo(1.45, 5);
		expect(damageRatioOf(500, 1000)).toBeCloseTo(0.5, 5);
		expect(damageRatioOf(900, 900)).toBe(1);
	});

	it('is unbounded when damage was dealt but none taken', () => {
		// NOT the dealt total: damage is in the hundreds-to-thousands while the
		// ratio sits near 1, so returning 900 here would break the column scale.
		expect(damageRatioOf(900, 0)).toBe(Infinity);
	});

	it('is zero before anyone has dealt anything', () => {
		expect(damageRatioOf(0, 0)).toBe(0);
	});
});

describe('applyIdentities', () => {
	const players = [player({ name: 'Stewball32' }), player({ name: 'CmdrKeyes' })];

	it('identified → default gamertag + avatar', () => {
		const [a] = applyIdentities(players, {
			stewball32: { display: 'Stewart', avatar: 'https://pb/av.png' }
		});
		expect(a.display).toBe('Stewart');
		expect(a.avatar).toBe('https://pb/av.png');
	});

	it('unidentified → trimmed scraped name + placeholder', () => {
		const [, b] = applyIdentities(players, { stewball32: { display: 'Stewart' } });
		expect(b.display).toBe('CmdrKeyes');
		expect(b.avatar).toBeNull();
	});

	it('identified without an avatar keeps the name swap and the placeholder', () => {
		const [a] = applyIdentities(players, { stewball32: { display: 'Stewart' } });
		expect(a.display).toBe('Stewart');
		expect(a.avatar).toBeNull();
	});

	it('carries motto + plate art onto the plate fields (settings Stream tab)', () => {
		const [a, b] = applyIdentities(players, {
			stewball32: {
				display: 'Stewart',
				motto: 'No tunnel is safe from me',
				plate: 'https://pb/api/files/nameplates/x/neon.png'
			}
		});
		expect(a.motto).toBe('No tunnel is safe from me');
		expect(a.plateBg).toBe('https://pb/api/files/nameplates/x/neon.png');
		// Unidentified (or motto-less) players get the empty defaults, never
		// undefined — the plate renders nothing rather than "undefined".
		expect(b.motto).toBe('');
		expect(b.plateBg).toBe('');
	});

	it('?names= override beats the resolved identity', () => {
		const [a] = applyIdentities(
			players,
			{ stewball32: { display: 'Stewart', avatar: 'https://pb/av.png' } },
			{ Stewball32: 'THE HOST' }
		);
		expect(a.display).toBe('THE HOST');
		// The override is a name, not an identity — the avatar still resolves.
		expect(a.avatar).toBe('https://pb/av.png');
	});

	it('never rewrites `name` — it is the lookup and animation key', () => {
		const [a] = applyIdentities(players, { stewball32: { display: 'Stewart' } });
		expect(a.name).toBe('Stewball32');
	});

	it('matches case-insensitively on the scraped name', () => {
		const [a] = applyIdentities([player({ name: 'STEWBALL32' })], {
			stewball32: { display: 'Stewart' }
		});
		expect(a.display).toBe('Stewart');
	});

	it('is a no-op with no identities', () => {
		expect(applyIdentities(players).map((p) => p.display)).toEqual(['Stewball32', 'CmdrKeyes']);
	});
});

// The respawn ring sweeps its arc off `respawn` and PlayerCard reads any
// `camo` above 1 as a literal percent-remaining. Both were previously
// quantised here (ceil to whole seconds; a constant 30), which froze the two
// animations without changing anything the older assertions looked at.
describe('live-tick vitals mapping', () => {
	const liveTick = (over: Record<string, unknown>) =>
		({
			players: [{ index: 0, alive: true, health: 1, shields: 1, ...over }],
			power_items: [],
			ctf_flags: [],
			game_globals: null,
			locals: []
		}) as unknown as TickPayloadV2;

	it('keeps respawn seconds fractional', () => {
		const [p] = overlayPlayers(gameWith(), liveTick({ alive: false, respawn_in_ticks: 91 }));
		expect(p.respawn).toBeCloseTo(91 / 30, 5);
	});

	it('is 0 seconds when the tick carries no respawn timer', () => {
		const [p] = overlayPlayers(gameWith(), liveTick({ respawn_in_ticks: null }));
		expect(p.respawn).toBe(0);
	});

	it('maps camo as a boolean, not a duration', () => {
		expect(overlayPlayers(gameWith(), liveTick({ has_camo: true }))[0].camo).toBe(1);
		expect(overlayPlayers(gameWith(), liveTick({ has_camo: false }))[0].camo).toBe(0);
	});
});

describe('scraped name trimming', () => {
	it('trims padding CE leaves on profile names', () => {
		const g = gameWith();
		g.players[0].name = '  Crazy  ';
		const [p] = overlayPlayers(g, null);
		expect(p.name).toBe('Crazy');
		expect(p.display).toBe('Crazy');
	});
});
