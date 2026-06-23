import { describe, it, expect } from 'vitest';
import { buildRooms, roomForPoint, classifyShellTriangles } from './rooms';

// Two spatially separated clusters of two triangles each: one near the origin,
// one ~100 units away on +X. k=2 should recover them.
function tri(ox: number, oy: number, oz: number) {
	return [ox, oy, oz, ox + 1, oy, oz, ox, oy + 1, oz];
}
const positions = [...tri(0, 0, 0), ...tri(0.5, 0.5, 0), ...tri(100, 0, 0), ...tri(100.5, 0.5, 0)];
const indices = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11];

describe('buildRooms', () => {
	it('partitions separated geometry into k rooms', () => {
		const rooms = buildRooms({ positions, indices }, 2);
		expect(rooms).toHaveLength(2);
		// Every triangle is assigned exactly once.
		const total = rooms.reduce((n, r) => n + r.triIndices.length, 0);
		expect(total).toBe(4);
	});

	it('rooms are indexed and sorted by centroid height', () => {
		const rooms = buildRooms({ positions, indices }, 2);
		expect(rooms.map((r) => r.index)).toEqual([0, 1]);
	});

	it('returns a single room when k=1 or geometry is tiny', () => {
		expect(buildRooms({ positions, indices }, 1)).toHaveLength(1);
		expect(buildRooms({ positions: tri(0, 0, 0), indices: [0, 1, 2] }, 4)).toHaveLength(1);
	});

	it('clusters only the included triangles when given an interior subset', () => {
		const rooms = buildRooms({ positions, indices }, 2, [0, 2]);
		const tris = rooms.flatMap((r) => r.triIndices).sort((a, b) => a - b);
		expect(tris).toEqual([0, 2]);
	});
});

describe('classifyShellTriangles', () => {
	// floor (up), ceiling (down), perimeter wall (x≈9), interior wall (x≈0).
	const pos = [
		0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0, 5, 0, 1, 5, 1, 0, 5, 9, 0, 0, 9, 1, 0, 9, 0, 1, 0, 0, 0, 0, 1,
		0, 0, 0, 1
	];
	const bounds = { minX: -10, maxX: 10, minY: -10, maxY: 10, minZ: 0, maxZ: 5 };

	it('culls ceilings + perimeter walls, keeps floors + interior walls (enclosed)', () => {
		const shell = classifyShellTriangles(
			{ positions: pos, indices: [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11], bounds },
			{ margin: 0.18 }
		);
		expect(shell).toEqual([false, true, true, false]);
	});

	it('keeps everything on an OPEN map (low wall fraction → not enclosed)', () => {
		// Three floors + one perimeter wall → 25% walls → open: nothing is culled,
		// so the boundary terrain survives (the Blood Gulch case).
		const openPos = [
			// three floor tris at varying XY
			0, 0, 0, 1, 0, 0, 0, 1, 0, 3, 3, 0, 4, 3, 0, 3, 4, 0, -5, -5, 0, -4, -5, 0, -5, -4, 0,
			// one perimeter wall
			9, 0, 0, 9, 1, 0, 9, 0, 1
		];
		const shell = classifyShellTriangles({
			positions: openPos,
			indices: [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11],
			bounds
		});
		expect(shell).toEqual([false, false, false, false]);
	});
});

describe('roomForPoint', () => {
	it('maps a point to the room whose AABB contains it', () => {
		const rooms = buildRooms({ positions, indices }, 2);
		const near = roomForPoint(rooms, { x: 0.3, y: 0.3, z: 0 });
		const far = roomForPoint(rooms, { x: 100.3, y: 0.3, z: 0 });
		expect(near).toBeGreaterThanOrEqual(0);
		expect(far).toBeGreaterThanOrEqual(0);
		expect(near).not.toBe(far);
	});

	it('returns -1 when the point is outside every room', () => {
		const rooms = buildRooms({ positions, indices }, 2);
		expect(roomForPoint(rooms, { x: 0, y: 0, z: 1000 })).toBe(-1);
	});
});
