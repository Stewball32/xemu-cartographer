import { describe, it, expect } from 'vitest';
import { mockStagedModel } from './mock-map';
import type { BspMesh } from './game-geometry';

// Synthetic two-floor mesh: a 3×3 grid of floor triangles at z=0 and another at
// z=5, so the staged mock has two distinct elevation bands to spread players over.
function floorTri(ox: number, oy: number, z: number): number[] {
	return [ox, oy, z, ox + 1, oy, z, ox, oy + 1, z];
}
function twoFloorMesh(): BspMesh {
	const positions: number[] = [];
	const indices: number[] = [];
	let v = 0;
	for (const z of [0, 5]) {
		for (let gx = 0; gx < 3; gx++) {
			for (let gy = 0; gy < 3; gy++) {
				positions.push(...floorTri(gx * 2, gy * 2, z));
				indices.push(v, v + 1, v + 2);
				v += 3;
			}
		}
	}
	return {
		game: 'haloce',
		scenario: 'levels\\test\\synthetic\\synthetic',
		sourceMap: 'synthetic.map',
		bounds: { minX: 0, maxX: 5, minY: 0, maxY: 5, minZ: 0, maxZ: 5 },
		positions,
		indices
	};
}

describe('mockStagedModel', () => {
	const mesh = twoFloorMesh();
	const m = mockStagedModel(mesh, mesh.scenario);

	it('stages the requested number of players, two teams, all alive', () => {
		expect(m.players).toHaveLength(8);
		expect(new Set(m.players.map((p) => p.team))).toEqual(new Set([0, 1]));
		expect(m.players.every((p) => p.alive)).toBe(true);
	});

	it('places players on BOTH floor bands (different elevations)', () => {
		const zs = m.players.map((p) => p.pos.z);
		expect(zs.some((z) => z < 2)).toBe(true); // lower floor
		expect(zs.some((z) => z > 3)).toBe(true); // upper floor
	});

	it('keeps every body within the mesh bounds and finite', () => {
		for (const p of m.players) {
			expect(Number.isFinite(p.pos.x)).toBe(true);
			expect(Number.isFinite(p.pos.y)).toBe(true);
			expect(Number.isFinite(p.pos.z)).toBe(true);
			expect(p.pos.x).toBeGreaterThanOrEqual(mesh.bounds.minX - 1);
			expect(p.pos.x).toBeLessThanOrEqual(mesh.bounds.maxX + 1);
		}
	});

	it('marks the demo as a live team game with a local player', () => {
		expect(m.phase).toBe('live');
		expect(m.isTeamGame).toBe(true);
		expect(m.players.filter((p) => p.isLocal)).toHaveLength(1);
	});
});
