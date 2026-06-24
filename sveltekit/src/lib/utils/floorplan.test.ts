import { describe, it, expect } from 'vitest';
import { buildFloorplan } from './floorplan';

// A large open floor (z=0) wound CCW (normal up) + green material colour.
const FLOOR = [0, 0, 0, 4, 0, 0, 0, 4, 0];
const FLOOR_COL = [0, 0.6, 0, 0, 0.6, 0, 0, 0.6, 0];

describe('buildFloorplan', () => {
	it('keeps an open floor as a walkable region carrying its material colour', () => {
		const fp = buildFloorplan({ positions: FLOOR, indices: [0, 1, 2], colors: FLOOR_COL });
		expect(fp.floors).toHaveLength(1);
		expect(fp.floors[0].color[1]).toBeCloseTo(0.6);
		expect(fp.floorZs).toEqual([0]);
	});

	it('drops a floor sealed under a low ceiling (crawlspace / inaccessible)', () => {
		// floor z=0 + ceiling z=1 over the same XY → no head clearance.
		const pos = [...FLOOR, 0, 0, 1, 0, 4, 1, 4, 0, 1];
		const fp = buildFloorplan({ positions: pos, indices: [0, 1, 2, 3, 4, 5] });
		expect(fp.floors).toHaveLength(0);
	});

	it('keeps a floor with adequate headroom (ceiling well above)', () => {
		const pos = [...FLOOR, 0, 0, 3, 0, 4, 3, 4, 0, 3];
		const fp = buildFloorplan({ positions: pos, indices: [0, 1, 2, 3, 4, 5] });
		expect(fp.floors).toHaveLength(1);
	});

	it('drops ceilings, emits clean wall/boundary outline segments for a walkable floor', () => {
		// open floor + a vertical wall along its x=4 edge.
		const pos = [...FLOOR, 4, 0, 0, 4, 2, 0, 4, 0, 2];
		const fp = buildFloorplan({ positions: pos, indices: [0, 1, 2, 3, 4, 5] });
		expect(fp.floors).toHaveLength(1);
		expect(fp.walls.length).toBeGreaterThan(0);
		// Segments, not filled tris: each has endpoints a/b + a height.
		expect(fp.walls[0].a).toBeDefined();
		expect(fp.walls[0].b).toBeDefined();
		expect(typeof fp.walls[0].z).toBe('number');
	});

	it('culls roof tops above the highest spawn + margin (spawn heuristic)', () => {
		// low walkable floor (z=0) + an exposed "roof" floor (z=10, up-facing). With
		// the highest spawn at z=0 (+2.5 margin), the z=10 roof must be culled.
		const ROOF = [0, 0, 10, 1, 0, 10, 0, 1, 10];
		const fp = buildFloorplan(
			{ positions: [...FLOOR, ...ROOF], indices: [0, 1, 2, 3, 4, 5] },
			{ spawnMaxZ: 0 }
		);
		expect(fp.floorZs).toEqual([0]); // only the low floor survives
	});

	it('guard: keeps an INTERIOR upper platform with no spawn (has a ceiling above)', () => {
		// low floor z=0 (spawn) + upper floor z=5 + a ceiling z=8 over the upper one.
		const UP = [0, 0, 5, 1, 0, 5, 0, 1, 5];
		const CEIL = [0, 0, 8, 0, 1, 8, 1, 0, 8]; // down-facing
		const fp = buildFloorplan(
			{ positions: [...FLOOR, ...UP, ...CEIL], indices: [0, 1, 2, 3, 4, 5, 6, 7, 8] },
			{ spawnMaxZ: 0 }
		);
		// Both floors kept: the upper one is interior play space, not a roof top.
		expect(fp.floorZs).toEqual([0, 5]);
	});

	it('is resilient to out-of-range indices', () => {
		const fp = buildFloorplan({ positions: [0, 0, 0], indices: [0, 1, 2] });
		expect(fp.floors).toHaveLength(0);
		expect(fp.walls).toHaveLength(0);
	});
});
