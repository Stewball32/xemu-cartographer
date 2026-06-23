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
export function buildRooms(
	mesh: Pick<BspMesh, 'positions' | 'indices'>,
	k: number,
	includeTris?: number[]
): Room[] {
	const cs = triCentroids(mesh);
	const triCount = cs.length;
	if (triCount === 0) return [];

	// Cluster only `includeTris` when given (the interior set, after the exterior
	// shell is culled), else every triangle. `work[j]` is the real triangle ordinal.
	const work =
		includeTris && includeTris.length > 0
			? includeTris.filter((t) => t >= 0 && t < triCount)
			: Array.from({ length: triCount }, (_, i) => i);
	const n = work.length;
	if (n === 0) return [];
	const pts = work.map((t) => cs[t]);
	const kk = Math.max(1, Math.min(k, n));

	const centroids = seed(pts, kk);
	const assign = new Array(n).fill(0);

	for (let iter = 0; iter < 30; iter++) {
		let moved = false;
		for (let j = 0; j < n; j++) {
			let best = 0;
			let bestD = Infinity;
			for (let c = 0; c < kk; c++) {
				const dx = pts[j].x - centroids[c].x;
				const dy = pts[j].y - centroids[c].y;
				const dz = (pts[j].z - centroids[c].z) * Z_WEIGHT;
				const d = dx * dx + dy * dy + dz * dz;
				if (d < bestD) {
					bestD = d;
					best = c;
				}
			}
			if (assign[j] !== best) {
				assign[j] = best;
				moved = true;
			}
		}
		const sx = new Array(kk).fill(0);
		const sy = new Array(kk).fill(0);
		const sz = new Array(kk).fill(0);
		const cnt = new Array(kk).fill(0);
		for (let j = 0; j < n; j++) {
			const c = assign[j];
			sx[c] += pts[j].x;
			sy[c] += pts[j].y;
			sz[c] += pts[j].z;
			cnt[c] += 1;
		}
		for (let c = 0; c < kk; c++) {
			if (cnt[c] > 0) {
				centroids[c] = { x: sx[c] / cnt[c], y: sy[c] / cnt[c], z: sz[c] / cnt[c] };
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
		for (let j = 0; j < n; j++) {
			if (assign[j] !== c) continue;
			const t = work[j];
			tris.push(t);
			for (let v = 0; v < 3; v++) {
				const o = idx[t * 3 + v] * 3;
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
 * Per-triangle EXTERIOR-SHELL classification — the outer boundary that blocks the
 * view from outside, culled so a spectator sees straight into the interior. A
 * triangle is shell when it's either:
 *   - a CEILING / roof underside (normal points down) — "cut the roof", OR
 *   - a near-vertical WALL whose centroid hugs the XY perimeter — the outer walls.
 * FLOORS are never shell (you stand on them; they don't block side views), and
 * interior walls (away from the perimeter) stay so the rooms keep their structure.
 *
 * This is position-based on purpose: in an enclosed map the engine only renders
 * the inward-facing side of every wall, so exterior and interior walls share a
 * normal direction — what distinguishes the shell is sitting at the boundary.
 * Returns a boolean per triangle ordinal (index/3).
 */
export function classifyShellTriangles(
	mesh: Pick<BspMesh, 'positions' | 'indices' | 'bounds'>,
	opts: { margin?: number; verticalT?: number; enclosedWallFrac?: number } = {}
): boolean[] {
	const margin = opts.margin ?? 0.18;
	const verticalT = opts.verticalT ?? 0.4;
	// Enclosed when the map is mostly walls (a boxed interior). Wall-ness uses the
	// ABSOLUTE vertical normal component, so it's independent of the BSP's (often
	// inconsistent) triangle winding — unlike the floor/ceiling sign. Real splits:
	// Chill Out 0.60, Prisoner 0.56 (enclosed) vs Blood Gulch 0.30 (open terrain).
	const enclosedWallFrac = opts.enclosedWallFrac ?? 0.45;
	const { positions: pos, indices: idx, bounds } = mesh;
	const mx = Math.max(1e-6, bounds.maxX - bounds.minX) * margin;
	const my = Math.max(1e-6, bounds.maxY - bounds.minY) * margin;

	// unit normal vertical component for a triangle (0 if degenerate / OOB).
	const unzOf = (i: number): number => {
		const a = idx[i] * 3;
		const b = idx[i + 1] * 3;
		const c = idx[i + 2] * 3;
		if (a + 2 >= pos.length || b + 2 >= pos.length || c + 2 >= pos.length) return 0;
		const ux = pos[b] - pos[a],
			uy = pos[b + 1] - pos[a + 1],
			uz = pos[b + 2] - pos[a + 2];
		const vx = pos[c] - pos[a],
			vy = pos[c + 1] - pos[a + 1],
			vz = pos[c + 2] - pos[a + 2];
		const nz = ux * vy - uy * vx;
		const len = Math.hypot(uy * vz - uz * vy, uz * vx - ux * vz, nz) || 1;
		return nz / len;
	};

	// First pass: wall fraction → enclosed? OPEN maps (e.g. Blood Gulch) keep ALL
	// their geometry — their boundary IS the playable map, and the winding-flipped
	// normals would otherwise mass-cull the terrain as bogus "ceilings".
	let wallish = 0;
	let total = 0;
	for (let i = 0; i + 2 < idx.length; i += 3) {
		total++;
		if (Math.abs(unzOf(i)) < verticalT) wallish++;
	}
	const enclosed = total > 0 && wallish / total >= enclosedWallFrac;
	if (!enclosed) return new Array(Math.floor(idx.length / 3)).fill(false);

	const out: boolean[] = [];
	for (let i = 0; i + 2 < idx.length; i += 3) {
		const a = idx[i] * 3;
		const b = idx[i + 1] * 3;
		const c = idx[i + 2] * 3;
		if (a + 2 >= pos.length || b + 2 >= pos.length || c + 2 >= pos.length) {
			out.push(false);
			continue;
		}
		const unz = unzOf(i);
		// Ceiling / roof underside → cull (cut the roof). Reliable enough on these
		// enclosed indoor maps (consistent winding); open maps already bailed above.
		if (unz <= -verticalT) {
			out.push(true);
			continue;
		}
		// Near-vertical wall hugging the XY perimeter → exterior wall → cull.
		const isWall = Math.abs(unz) < verticalT;
		const cx = (pos[a] + pos[b] + pos[c]) / 3;
		const cy = (pos[a + 1] + pos[b + 1] + pos[c + 1]) / 3;
		const nearPerim =
			cx <= bounds.minX + mx ||
			cx >= bounds.maxX - mx ||
			cy <= bounds.minY + my ||
			cy >= bounds.maxY - my;
		out.push(isWall && nearPerim);
	}
	return out;
}
