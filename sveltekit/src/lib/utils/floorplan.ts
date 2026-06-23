// Floorplan extraction from the structure-BSP mesh — the 2D "cut the roof off"
// render. Pure / IO-free so the per-triangle classification (the part that has
// to be CORRECT) unit-tests without a browser.
//
// We classify every triangle by its world-space normal's vertical component:
//   - floor   : normal points UP   (nz >= +T) — walkable surface, FILLED
//   - ceiling : normal points DOWN (nz <= -T) — the roof, DROPPED (that's the cut)
//   - wall    : near-vertical       (|nz| < T) — drawn as a thin OUTLINE
// Dropping ceilings is exactly what lets a spectator see into rooms top-down;
// floors carry the elevation shading, walls give the room outlines.
//
// Coordinates: Halo world X/Y is the ground plane, Z up (same frame as the live
// markers). Floor/wall geometry is emitted in world XY (+ carried Z) so the 2D
// projection places it through the SAME transform the dots use → aligned.

import type { BspMesh } from '$lib/utils/game-geometry';
import type { Vec2 } from '$lib/types/scraper-v2';

export interface FloorTri {
	/** Triangle corners in world XY. */
	pts: [Vec2, Vec2, Vec2];
	/** Average world Z (height) of the triangle — drives elevation shading. */
	z: number;
}

export interface WallSeg {
	a: Vec2;
	b: Vec2;
	/** Higher of the two endpoints' Z — lets walls be drawn back-to-front. */
	z: number;
}

export interface Floorplan {
	floors: FloorTri[];
	walls: WallSeg[];
	/** All floor-triangle mid-Z values (the input to elevation banding). */
	floorZs: number[];
}

/** Cosine threshold separating floors/ceilings from walls. A normal whose |nz|
 *  exceeds this is treated as horizontal (floor/ceiling); below it, vertical. */
const VERTICAL_T = 0.5;

function cross(
	ax: number,
	ay: number,
	az: number,
	bx: number,
	by: number,
	bz: number
): [number, number, number] {
	return [ay * bz - az * by, az * bx - ax * bz, ax * by - ay * bx];
}

/**
 * Build the top-down floorplan from a BSP mesh: floors (filled), walls
 * (outlined), ceilings dropped. Floors are sorted low-Z first so that when the
 * consumer paints them in order, higher walkable surfaces land on top — the
 * correct read for stacked floors in plan view.
 */
export function buildFloorplan(mesh: Pick<BspMesh, 'positions' | 'indices'>): Floorplan {
	const pos = mesh.positions;
	const idx = mesh.indices;
	const floors: FloorTri[] = [];
	const walls: WallSeg[] = [];
	const floorZs: number[] = [];

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

		const n = cross(bx - ax, by - ay, bz - az, cx - ax, cy - ay, cz - az);
		const len = Math.hypot(n[0], n[1], n[2]);
		if (len < 1e-9) continue;
		const nz = n[2] / len; // vertical component of the unit normal

		if (nz >= VERTICAL_T) {
			// Floor (normal up).
			const z = (az + bz + cz) / 3;
			floors.push({
				pts: [
					{ x: ax, y: ay },
					{ x: bx, y: by },
					{ x: cx, y: cy }
				],
				z
			});
			floorZs.push(z);
		} else if (nz <= -VERTICAL_T) {
			// Ceiling — dropped (the cut).
			continue;
		} else {
			// Wall — emit its top edge (the two highest vertices) as an outline.
			const verts = [
				{ x: ax, y: ay, z: az },
				{ x: bx, y: by, z: bz },
				{ x: cx, y: cy, z: cz }
			].sort((p, q) => q.z - p.z);
			walls.push({
				a: { x: verts[0].x, y: verts[0].y },
				b: { x: verts[1].x, y: verts[1].y },
				z: verts[0].z
			});
		}
	}

	floors.sort((p, q) => p.z - q.z);
	walls.sort((p, q) => p.z - q.z);
	return { floors, walls, floorZs };
}
