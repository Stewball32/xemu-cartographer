// Architectural floorplan extraction from the structure-BSP mesh — the 2D "clean
// overhead map" render (north star: KUUOMA's Chill Out overhead map — solid
// filled rooms + crisp wall outlines, NOT a wireframe of every BSP edge). Pure /
// IO-free so the classification + walkable detection unit-test without a browser.
//
// We keep only the WALKABLE space and the walls that bound it:
//   - floor  : up-facing surface (nz >= +T) WITH head clearance above (no ceiling
//              within ~player height) — the walkable regions, FILLED in real
//              material colour + elevation shade.
//   - ceiling: down-facing (nz <= -T) — DROPPED (cut the roof).
//   - wall   : near-vertical (|nz| < T) that bounds a walkable region — FILLED as
//              a thin sliver (its top-down footprint), giving clean room outlines.
//   - everything else (sealed voids, structural backing under a low ceiling, geo
//     outside the playable space) is culled by the clearance + adjacency tests.
//
// Coordinates: Halo world X/Y is the ground plane, Z up (same frame as the live
// markers), so the 2D projection places floors/walls through the SAME transform
// the dots use → aligned.

import type { BspMesh } from '$lib/utils/game-geometry';
import type { Vec2 } from '$lib/types/scraper-v2';

export interface FloorTri {
	/** Triangle corners in world XY. */
	pts: [Vec2, Vec2, Vec2];
	/** Average world Z (height) of the triangle — drives elevation shading. */
	z: number;
	/** Texture-sampled material colour (rgb 0..1) for this surface. */
	color: [number, number, number];
}

export interface WallSeg {
	a: Vec2;
	b: Vec2;
	/** Representative Z (for height banding). */
	z: number;
}

export interface Floorplan {
	floors: FloorTri[];
	/** Clean architectural outlines: the boundary of the walkable region + the
	 *  footprints of the vertical walls that divide it (deduped → no edge clutter). */
	walls: WallSeg[];
	/** Walkable-floor mid-Z values (the input to elevation banding). */
	floorZs: number[];
}

/** Cosine threshold separating floors/ceilings from walls. */
const VERTICAL_T = 0.5;
/** Headroom a player needs above a floor for it to count as walkable (world u). */
const HEAD_H = 2.2;
/** Surfaces within this of a floor are treated as coplanar, not a ceiling. */
const CLEAR_MIN = 0.6;
const NEUTRAL: [number, number, number] = [0.5, 0.5, 0.52];

interface Tri {
	pts: [Vec2, Vec2, Vec2];
	z: number;
	/** Lowest corner Z — used to drop structural walls floating above the floors. */
	zMin: number;
	color: [number, number, number];
	minX: number;
	maxX: number;
	minY: number;
	maxY: number;
}

function triColorAt(
	colors: number[] | undefined,
	v0: number,
	v1: number,
	v2: number
): [number, number, number] {
	if (!colors || v0 + 2 >= colors.length) return NEUTRAL;
	// Average the three vertex colours so a flat-shaded surface reads its material.
	const r = (colors[v0] + colors[v1] + colors[v2]) / 3;
	const g = (colors[v0 + 1] + colors[v1 + 1] + colors[v2 + 1]) / 3;
	const b = (colors[v0 + 2] + colors[v1 + 2] + colors[v2 + 2]) / 3;
	if (!Number.isFinite(r)) return NEUTRAL;
	return [r, g, b];
}

/**
 * Build the architectural floorplan: filled walkable floors (in material colour)
 * + the walls bounding them, with ceilings + inaccessible geometry culled.
 */
export function buildFloorplan(
	mesh: Pick<BspMesh, 'positions' | 'indices' | 'colors'>,
	opts?: { spawnMaxZ?: number; margin?: number }
): Floorplan {
	const pos = mesh.positions;
	const idx = mesh.indices;
	const colors = mesh.colors;

	const floorCand: Tri[] = [];
	const wallCand: Tri[] = [];
	// Horizontal surfaces (floors + ceilings) → blockers for the clearance test.
	const horiz: { z: number; minX: number; maxX: number; minY: number; maxY: number }[] = [];

	let minX = Infinity,
		maxX = -Infinity,
		minY = Infinity,
		maxY = -Infinity;

	for (let i = 0; i + 2 < idx.length; i += 3) {
		const a = idx[i] * 3;
		const b = idx[i + 1] * 3;
		const c = idx[i + 2] * 3;
		if (a + 2 >= pos.length || b + 2 >= pos.length || c + 2 >= pos.length) continue;

		const ax = pos[a],
			ay = pos[a + 1],
			az = pos[a + 2];
		const bx = pos[b],
			by = pos[b + 1],
			bz = pos[b + 2];
		const cx = pos[c],
			cy = pos[c + 1],
			cz = pos[c + 2];

		const nx = (by - ay) * (cz - az) - (bz - az) * (cy - ay);
		const ny = (bz - az) * (cx - ax) - (bx - ax) * (cz - az);
		const nz = (bx - ax) * (cy - ay) - (by - ay) * (cx - ax);
		const len = Math.hypot(nx, ny, nz);
		if (len < 1e-9) continue;
		const unz = nz / len;

		const tMinX = Math.min(ax, bx, cx),
			tMaxX = Math.max(ax, bx, cx);
		const tMinY = Math.min(ay, by, cy),
			tMaxY = Math.max(ay, by, cy);
		if (tMinX < minX) minX = tMinX;
		if (tMaxX > maxX) maxX = tMaxX;
		if (tMinY < minY) minY = tMinY;
		if (tMaxY > maxY) maxY = tMaxY;

		const pts: [Vec2, Vec2, Vec2] = [
			{ x: ax, y: ay },
			{ x: bx, y: by },
			{ x: cx, y: cy }
		];
		const color = triColorAt(colors, a, b, c);

		if (Math.abs(unz) >= VERTICAL_T) {
			// Horizontal surface (floor or ceiling) — both block clearance.
			const zAvg = (az + bz + cz) / 3;
			horiz.push({ z: zAvg, minX: tMinX, maxX: tMaxX, minY: tMinY, maxY: tMaxY });
			if (unz >= VERTICAL_T) {
				floorCand.push({
					pts,
					z: zAvg,
					zMin: Math.min(az, bz, cz),
					color,
					minX: tMinX,
					maxX: tMaxX,
					minY: tMinY,
					maxY: tMaxY
				});
			}
			// ceilings (unz <= -T) are dropped — not added to floorCand.
		} else {
			wallCand.push({
				pts,
				z: Math.max(az, bz, cz),
				zMin: Math.min(az, bz, cz),
				color,
				minX: tMinX,
				maxX: tMaxX,
				minY: tMinY,
				maxY: tMaxY
			});
		}
	}

	if (!Number.isFinite(minX) || floorCand.length === 0) {
		return { floors: [], walls: [], floorZs: [] };
	}

	// --- Clearance grid: bucket every horizontal surface into the cells its XY
	//     bbox covers, so a floor can ask "is anything overhead within HEAD_H?" ---
	const span = Math.max(maxX - minX, maxY - minY);
	const G = Math.max(0.5, span / 48);
	const cellsForBbox = (t: { minX: number; maxX: number; minY: number; maxY: number }) => {
		const gx0 = Math.floor((t.minX - minX) / G);
		const gx1 = Math.floor((t.maxX - minX) / G);
		const gy0 = Math.floor((t.minY - minY) / G);
		const gy1 = Math.floor((t.maxY - minY) / G);
		return { gx0, gx1, gy0, gy1 };
	};
	const blockers = new Map<string, number[]>();
	for (const h of horiz) {
		const { gx0, gx1, gy0, gy1 } = cellsForBbox(h);
		for (let gx = gx0; gx <= gx1; gx++)
			for (let gy = gy0; gy <= gy1; gy++) {
				const k = `${gx},${gy}`;
				const arr = blockers.get(k);
				if (arr) arr.push(h.z);
				else blockers.set(k, [h.z]);
			}
	}

	// --- Walkable floors: a floor with no surface in (z+CLEAR_MIN, z+HEAD_H) over
	//     any cell it covers (i.e. real headroom, not a sealed crawlspace). ---
	const walkable: Tri[] = [];
	for (const f of floorCand) {
		const { gx0, gx1, gy0, gy1 } = cellsForBbox(f);
		let blocked = false;
		for (let gx = gx0; gx <= gx1 && !blocked; gx++)
			for (let gy = gy0; gy <= gy1 && !blocked; gy++) {
				const arr = blockers.get(`${gx},${gy}`);
				if (!arr) continue;
				for (const z of arr) {
					if (z > f.z + CLEAR_MIN && z < f.z + HEAD_H) {
						blocked = true;
						break;
					}
				}
			}
		if (blocked) continue;
		walkable.push(f);
	}

	if (walkable.length === 0) {
		return { floors: [], walls: [], floorZs: [] };
	}

	// --- Roof/ceiling cull by SPAWN HEIGHT (Stewart's heuristic). The up-facing
	//     TOP of a roof survives the clearance test (nothing is above it), so it
	//     renders as a bogus high "floor". Spawns only ever sit on real play space,
	//     so anything above the highest spawn + a headroom margin (jump/standing
	//     clearance) is roof and gets culled. GUARD: never clip an INTERIOR walkable
	//     floor (one with a ceiling/upper-floor above it = enclosed play space) —
	//     raise the cap to keep it, so a no-spawn upper platform isn't lost. ---
	const MARGIN = opts?.margin ?? 2.5;
	const hasSurfaceAbove = (f: Tri): boolean => {
		const { gx0, gx1, gy0, gy1 } = cellsForBbox(f);
		for (let gx = gx0; gx <= gx1; gx++)
			for (let gy = gy0; gy <= gy1; gy++) {
				const arr = blockers.get(`${gx},${gy}`);
				if (!arr) continue;
				for (const z of arr) if (z > f.z + 1.0) return true;
			}
		return false;
	};
	const haveSpawnCap = opts?.spawnMaxZ != null && Number.isFinite(opts.spawnMaxZ);
	let highestInteriorZ = -Infinity;
	if (haveSpawnCap)
		for (const f of walkable)
			if (hasSurfaceAbove(f) && f.z > highestInteriorZ) highestInteriorZ = f.z;
	// Only cull when spawns give a real threshold. Without spawns we don't guess a
	// ceiling (the interior guard alone would over-cull a legit exposed top floor).
	const ceilingCap = haveSpawnCap
		? Math.max((opts!.spawnMaxZ as number) + MARGIN, highestInteriorZ)
		: Infinity;
	const capped = walkable.filter((f) => f.z <= ceilingCap);
	const useFloors = capped.length > 0 ? capped : walkable; // never blank the whole map

	let walkMaxZ = -Infinity;
	for (const f of useFloors) if (f.z > walkMaxZ) walkMaxZ = f.z;

	// --- Clean room outlines via RASTERIZE-THEN-TRACE. Per-triangle edges are
	//     hopeless on real BSP (Chill Out has ~1700 wall tris). Instead we rasterise
	//     the walkable area into a fine grid, carve the vertical-wall footprints out
	//     of it, and emit the grid's boundary segments — collapsing all that geometry
	//     into a handful of clean room/level silhouettes + interior wall lines. ---
	const RES = 200;
	const G2 = Math.max(0.08, Math.max(maxX - minX, maxY - minY) / RES);
	const cols = Math.max(1, Math.ceil((maxX - minX) / G2));
	const rows = Math.max(1, Math.ceil((maxY - minY) / G2));
	const cellZ = new Float32Array(cols * rows).fill(NaN);
	const cellWall = new Uint8Array(cols * rows);

	const inTri = (px: number, py: number, t: Tri): boolean => {
		const [a, b, c] = t.pts;
		const d = (b.y - c.y) * (a.x - c.x) + (c.x - b.x) * (a.y - c.y);
		if (Math.abs(d) < 1e-12) return false;
		const wa = ((b.y - c.y) * (px - c.x) + (c.x - b.x) * (py - c.y)) / d;
		const wb = ((c.y - a.y) * (px - c.x) + (a.x - c.x) * (py - c.y)) / d;
		const wc = 1 - wa - wb;
		return wa >= -0.02 && wb >= -0.02 && wc >= -0.02;
	};
	const rasterize = (t: Tri, set: (i: number) => void) => {
		const ci0 = Math.max(0, Math.floor((t.minX - minX) / G2));
		const ci1 = Math.min(cols - 1, Math.floor((t.maxX - minX) / G2));
		const cj0 = Math.max(0, Math.floor((t.minY - minY) / G2));
		const cj1 = Math.min(rows - 1, Math.floor((t.maxY - minY) / G2));
		for (let ci = ci0; ci <= ci1; ci++)
			for (let cj = cj0; cj <= cj1; cj++) {
				const px = minX + (ci + 0.5) * G2;
				const py = minY + (cj + 0.5) * G2;
				if (inTri(px, py, t)) set(cj * cols + ci);
			}
	};

	// Walkable cells take the TOP-most floor z that covers them (so an upper floor
	// reads over a lower one in plan view).
	for (const f of useFloors)
		rasterize(f, (idxc) => {
			if (Number.isNaN(cellZ[idxc]) || f.z > cellZ[idxc]) cellZ[idxc] = f.z;
		});
	// Carve vertical-wall footprints out of the mask — but only walls whose base is
	// within the (spawn-capped) playable band, so roof parapets don't draw.
	for (const w of wallCand) {
		if (w.zMin > ceilingCap) continue;
		rasterize(w, (idxc) => {
			cellWall[idxc] = 1;
		});
	}

	const isWalk = (ci: number, cj: number): boolean => {
		if (ci < 0 || cj < 0 || ci >= cols || cj >= rows) return false;
		const idxc = cj * cols + ci;
		return !Number.isNaN(cellZ[idxc]) && !cellWall[idxc];
	};
	const walls: WallSeg[] = [];
	const zAt = (ci: number, cj: number): number => {
		const idxc = cj * cols + ci;
		const z = cellZ[idxc];
		return Number.isNaN(z) ? walkMaxZ : z;
	};
	// Boundary = transition between a walkable and a non-walkable cell. Emit the
	// shared edge as a world-space segment (z from the walkable side, for banding).
	for (let cj = 0; cj < rows; cj++)
		for (let ci = 0; ci < cols; ci++) {
			const here = isWalk(ci, cj);
			if (here !== isWalk(ci + 1, cj)) {
				const x = minX + (ci + 1) * G2;
				walls.push({
					a: { x, y: minY + cj * G2 },
					b: { x, y: minY + (cj + 1) * G2 },
					z: here ? zAt(ci, cj) : zAt(ci + 1, cj)
				});
			}
			if (here !== isWalk(ci, cj + 1)) {
				const y = minY + (cj + 1) * G2;
				walls.push({
					a: { x: minX + ci * G2, y },
					b: { x: minX + (ci + 1) * G2, y },
					z: here ? zAt(ci, cj) : zAt(ci, cj + 1)
				});
			}
		}

	const floors: FloorTri[] = useFloors
		.map((f) => ({ pts: f.pts, z: f.z, color: f.color }))
		.sort((p, q) => p.z - q.z);
	walls.sort((p, q) => p.z - q.z);
	return { floors, walls, floorZs: floors.map((f) => f.z) };
}
