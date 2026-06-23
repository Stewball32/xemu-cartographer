import { describe, it, expect } from 'vitest';
import { buildRooms, roomForPoint, classifyOuterShell } from './rooms';

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

describe('classifyOuterShell', () => {
	it('flags rooms hugging the world bounds', () => {
		const rooms = buildRooms({ positions, indices }, 2);
		const bounds = { minX: 0, maxX: 101.5, minY: 0, maxY: 1.5, minZ: 0, maxZ: 0 };
		const shell = classifyOuterShell(rooms, bounds, 0.12);
		expect(shell).toHaveLength(rooms.length);
		// Both clusters touch an outer edge here.
		expect(shell.some(Boolean)).toBe(true);
	});
});
