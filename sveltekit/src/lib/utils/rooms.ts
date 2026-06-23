// Room ("BSP cluster") partitioning of the structure-BSP mesh — the unit the 3D
// occupancy reveal fades when a player steps inside, and the granularity the
// camera-occlusion + outer-shell passes operate on. Pure / IO-free so the
// clustering + the point-in-room test unit-test without a GPU.
//
// True engine BSP leaf-clusters aren't in the extracted cache (positions +
// indices + colors only), so we APPROXIMATE rooms by clustering triangle
// centroids: deterministic k-means in world XYZ with Z up-weighted, so vertically
// separated floors fall into different rooms (a stacked map reveals one floor at
// a time, not the whole column). This is a readability approximation — a cluster
// can straddle a thin wall — which is acceptable for the spectator overview and
// noted as such. Each room carries its triangle indices so the renderer can build
// one independently-fadeable sub-mesh per room.

import type { BspMesh } from '$lib/utils/game-geometry';
import type { Vec3 } from '$lib/types/scraper-v2';

export interface RoomAABB {
	minX: number;
	maxX: number;
	minY: number;
	maxY: number;
	minZ: number;
	maxZ: number;
}

export interface Room {
	index: number;
	/** Centroid of the room's triangle centroids (world coords). */
	center: Vec3;
	aabb: RoomAABB;
	/** Triangle ordinals (i.e. index/3) assigned to this room. */
	triIndices: number[];
}

/** Z weight: vertical separation counts 1.6× horizontal so floors split cleanly. */
const Z_WEIGHT = 1.6;

interface Centroid {
	x: number;
	y: number;
	z: number;
}

function triCentroids(mesh: Pick<BspMesh, 'positions' | 'indices'>): Centroid[] {
	const { positions: pos, indices: idx } = mesh;
	const out: Centroid[] = [];
	for (let i = 0; i + 2 < idx.length; i += 3) {
		const a = idx[i] * 3;
		const b = idx[i + 1] * 3;
		const c = idx[i + 2] * 3;
		if (a + 2 >= pos.length || b + 2 >= pos.length || c + 2 >= pos.length) {
			out.push({ x: 0, y: 0, z: 0 });
			continue;
		}
		out.push({
			x: (pos[a] + pos[b] + pos[c]) / 3,
			y: (pos[a + 1] + pos[b + 1] + pos[c + 1]) / 3,
			z: (pos[a + 2] + pos[b + 2] + pos[c + 2]) / 3
		});
	}
	return out;
}

/** Farthest-point seeding (deterministic: starts from triangle 0) so clusters
 *  spread across the map instead of clumping near a random init. */
function seed(cs: Centroid[], k: number): Centroid[] {
	const seeds: Centroid[] = [{ ...cs[0] }];
	while (seeds.length < k) {
		let bestI = 0;
		let bestD = -1;
		for (let i = 0; i < cs.length; i++) {
			let nearest = Infinity;
			for (const s of seeds) {
				const dx = cs[i].x - s.x;
				const dy = cs[i].y - s.y;
				const dz = (cs[i].z - s.z) * Z_WEIGHT;
				const d = dx * dx + dy * dy + dz * dz;
				if (d < nearest) nearest = d;
			}
			if (nearest > bestD) {
				bestD = nearest;
				bestI = i;
			}
		}
		seeds.push({ ...cs[bestI] });
	}
	return seeds;
}

/**
 * Partition a mesh into up to `k` rooms. Returns rooms sorted by their centroid
 * Z (lowest floor first), each with a tight AABB + its triangle ordinals. Empty
 * meshes / k<=1 yield a single room covering everything.
 */
export function buildRooms(mesh: Pick<BspMesh, 'positions' | 'indices'>, k: number): Room[] {
	const cs = triCentroids(mesh);
	const triCount = cs.length;
	if (triCount === 0) return [];
	const kk = Math.max(1, Math.min(k, triCount));

	const centroids = seed(cs, kk);
	const assign = new Array(triCount).fill(0);

	for (let iter = 0; iter < 30; iter++) {
		let moved = false;
		for (let i = 0; i < triCount; i++) {
			let best = 0;
			let bestD = Infinity;
			for (let c = 0; c < kk; c++) {
				const dx = cs[i].x - centroids[c].x;
				const dy = cs[i].y - centroids[c].y;
				const dz = (cs[i].z - centroids[c].z) * Z_WEIGHT;
				const d = dx * dx + dy * dy + dz * dz;
				if (d < bestD) {
					bestD = d;
					best = c;
				}
			}
			if (assign[i] !== best) {
				assign[i] = best;
				moved = true;
			}
		}
		const sx = new Array(kk).fill(0);
		const sy = new Array(kk).fill(0);
		const sz = new Array(kk).fill(0);
		const n = new Array(kk).fill(0);
		for (let i = 0; i < triCount; i++) {
			const c = assign[i];
			sx[c] += cs[i].x;
			sy[c] += cs[i].y;
			sz[c] += cs[i].z;
			n[c] += 1;
		}
		for (let c = 0; c < kk; c++) {
			if (n[c] > 0) {
				centroids[c] = { x: sx[c] / n[c], y: sy[c] / n[c], z: sz[c] / n[c] };
			}
		}
		if (!moved) break;
	}

	// Gather triangles per cluster + tight AABBs; drop empties.
	const { positions: pos, indices: idx } = mesh;
	const rooms: Room[] = [];
	for (let c = 0; c < kk; c++) {
		const tris: number[] = [];
		let minX = Infinity,
			maxX = -Infinity,
			minY = Infinity,
			maxY = -Infinity,
			minZ = Infinity,
			maxZ = -Infinity;
		for (let i = 0; i < triCount; i++) {
			if (assign[i] !== c) continue;
			tris.push(i);
			for (let v = 0; v < 3; v++) {
				const o = idx[i * 3 + v] * 3;
				if (o + 2 >= pos.length) continue;
				const x = pos[o],
					y = pos[o + 1],
					z = pos[o + 2];
				if (x < minX) minX = x;
				if (x > maxX) maxX = x;
				if (y < minY) minY = y;
				if (y > maxY) maxY = y;
				if (z < minZ) minZ = z;
				if (z > maxZ) maxZ = z;
			}
		}
		if (tris.length === 0) continue;
		rooms.push({
			index: 0,
			center: centroids[c],
			aabb: { minX, maxX, minY, maxY, minZ, maxZ },
			triIndices: tris
		});
	}

	rooms.sort((a, b) => a.center.z - b.center.z);
	rooms.forEach((r, i) => (r.index = i));
	return rooms;
}

function inflate(a: RoomAABB, s: number): RoomAABB {
	return {
		minX: a.minX - s,
		maxX: a.maxX + s,
		minY: a.minY - s,
		maxY: a.maxY + s,
		minZ: a.minZ - s,
		maxZ: a.maxZ + s
	};
}

/**
 * Which room a world point sits in: the nearest-centroid room among those whose
 * (slightly inflated) AABB contains the point, else -1. Inflation tolerates a
 * player standing a touch above the floor or just past a room's triangle extent.
 */
export function roomForPoint(rooms: Room[], p: Vec3, slack = 1.5): number {
	let best = -1;
	let bestD = Infinity;
	for (const r of rooms) {
		const a = inflate(r.aabb, slack);
		if (
			p.x < a.minX ||
			p.x > a.maxX ||
			p.y < a.minY ||
			p.y > a.maxY ||
			p.z < a.minZ ||
			p.z > a.maxZ
		)
			continue;
		const dx = p.x - r.center.x;
		const dy = p.y - r.center.y;
		const dz = p.z - r.center.z;
		const d = dx * dx + dy * dy + dz * dz;
		if (d < bestD) {
			bestD = d;
			best = r.index;
		}
	}
	return best;
}

/**
 * Outer-shell classification: rooms whose AABB hugs the world bounds on any axis
 * (within `margin` fraction of the span) form the map's enclosing shell. Toggling
 * these transparent is what opens up a fixed overview cam without touching the
 * interior rooms. Returns a boolean per room index.
 */
export function classifyOuterShell(rooms: Room[], bounds: RoomAABB, margin = 0.12): boolean[] {
	const spanX = Math.max(1e-6, bounds.maxX - bounds.minX);
	const spanY = Math.max(1e-6, bounds.maxY - bounds.minY);
	const spanZ = Math.max(1e-6, bounds.maxZ - bounds.minZ);
	const mx = spanX * margin;
	const my = spanY * margin;
	const mz = spanZ * margin;
	return rooms.map((r) => {
		const a = r.aabb;
		return (
			a.minX <= bounds.minX + mx ||
			a.maxX >= bounds.maxX - mx ||
			a.minY <= bounds.minY + my ||
			a.maxY >= bounds.maxY - my ||
			a.maxZ >= bounds.maxZ - mz
		);
	});
}
