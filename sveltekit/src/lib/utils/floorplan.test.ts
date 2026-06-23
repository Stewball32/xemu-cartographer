import { describe, it, expect } from 'vitest';
import { buildFloorplan } from './floorplan';

// Three triangles with known normals:
//   floor   — in the XY plane, wound CCW so the normal points +Z
//   ceiling — in a higher XY plane, wound CW so the normal points −Z (dropped)
//   wall    — in the XZ plane, normal horizontal
const positions = [
	// floor (z=0)
	0, 0, 0, 1, 0, 0, 0, 1, 0,
	// ceiling (z=5)
	0, 0, 5, 0, 1, 5, 1, 0, 5,
	// wall (xz plane)
	0, 0, 0, 1, 0, 0, 0, 0, 1
];
const indices = [0, 1, 2, 3, 4, 5, 6, 7, 8];

describe('buildFloorplan', () => {
	it('keeps floors, drops ceilings, outlines walls', () => {
		const fp = buildFloorplan({ positions, indices });
		expect(fp.floors).toHaveLength(1);
		expect(fp.walls).toHaveLength(1);
		expect(fp.floorZs).toEqual([0]);
	});

	it('carries the floor triangle in world XY with its height', () => {
		const fp = buildFloorplan({ positions, indices });
		const f = fp.floors[0];
		expect(f.z).toBeCloseTo(0);
		expect(f.pts).toHaveLength(3);
		expect(f.pts[1]).toEqual({ x: 1, y: 0 });
	});

	it('is resilient to out-of-range indices', () => {
		const fp = buildFloorplan({ positions: [0, 0, 0], indices: [0, 1, 2] });
		expect(fp.floors).toHaveLength(0);
		expect(fp.walls).toHaveLength(0);
	});

	it('sorts floors low-Z first so higher floors paint on top', () => {
		const pos = [
			// low floor z=0
			0, 0, 0, 1, 0, 0, 0, 1, 0,
			// high floor z=4
			0, 0, 4, 1, 0, 4, 0, 1, 4
		];
		const fp = buildFloorplan({ positions: pos, indices: [0, 1, 2, 3, 4, 5] });
		expect(fp.floors.map((f) => f.z)).toEqual([0, 4]);
	});
});
