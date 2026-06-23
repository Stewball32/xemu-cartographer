import { describe, it, expect } from 'vitest';
import { mockMapModel, type BoundsLike } from './mock-map';

// Real extracted world-bounds for three very different-scale maps.
const BLOOD_GULCH: BoundsLike = {
	minX: 5.9,
	maxX: 131.9,
	minY: -190.2,
	maxY: -45.1,
	minZ: -0.3,
	maxZ: 26.2
};
const CHILL_OUT: BoundsLike = {
	minX: -10.7,
	maxX: 12.8,
	minY: -8.2,
	maxY: 13.0,
	minZ: -2.5,
	maxZ: 8.9
};
const PRISONER: BoundsLike = {
	minX: -12.6,
	maxX: 11.0,
	minY: -7.0,
	maxY: 7.6,
	minZ: -3.2,
	maxZ: 9.2
};

function within(p: { x: number; y: number; z: number }, b: BoundsLike, slack = 1e-6): boolean {
	return (
		p.x >= b.minX - slack && p.x <= b.maxX + slack && p.y >= b.minY - slack && p.y <= b.maxY + slack
	);
}

describe('mockMapModel', () => {
	for (const [name, b] of [
		['Blood Gulch', BLOOD_GULCH],
		['Chill Out', CHILL_OUT],
		['Prisoner', PRISONER]
	] as const) {
		it(`places a 4v4 with every marker inside ${name}'s real bounds`, () => {
			const m = mockMapModel(b, 'levels\\test\\x\\x');
			expect(m.players.length).toBe(8);
			expect(new Set(m.players.map((p) => p.team))).toEqual(new Set([0, 1]));
			for (const p of m.players) expect(within(p.pos, b)).toBe(true);
			for (const it of m.items) expect(within(it.pos, b)).toBe(true);
		});
	}

	it('sets bounds to the EXACT input bounds (valid/static) so the map background + camera align', () => {
		const m = mockMapModel(CHILL_OUT, 'chillout');
		expect(m.bounds).toMatchObject({ ...CHILL_OUT, valid: true, source: 'static' });
	});

	it('marks the demo as a live team game with positions', () => {
		const m = mockMapModel(PRISONER, 'prisoner');
		expect(m.isTeamGame).toBe(true);
		expect(m.phase).toBe('live');
		expect(m.hasPositions).toBe(true);
		expect(m.placedCount).toBe(8);
	});

	it('derives a display map name from the scenario tag path', () => {
		expect(mockMapModel(CHILL_OUT, 'levels\\test\\chillout\\chillout').mapName).toBe('Chillout');
	});

	it('gives each player a heading (faces the enemy base)', () => {
		const m = mockMapModel(BLOOD_GULCH, 'bloodgulch');
		for (const p of m.players) expect(p.heading == null || Number.isFinite(p.heading)).toBe(true);
		expect(m.players.some((p) => p.heading != null)).toBe(true);
	});
});
