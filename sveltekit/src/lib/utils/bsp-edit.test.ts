import { describe, it, expect } from 'vitest';
import type { BspMesh } from './game-geometry';
import {
	triCount,
	buildTriMeta,
	connectedComponents,
	selectAbovePlane,
	selectComponent,
	selectByMaterial,
	boxSelect,
	pointInRect,
	exportSpectatorMesh,
	editStats,
	RemovedHistory,
	defaultCullZ
} from './bsp-edit';

/** Two disconnected quads (4 triangles) at two heights, with two materials.
 *  - tris 0,1: a floor quad at z=0, red, in the XY range [0..2]×[0..2]
 *  - tris 2,3: a ceiling quad at z=10, blue, in the XY range [5..7]×[0..2]
 *  Vertices are duplicated per-quad so welding-based connectivity is exercised. */
function twoQuadMesh(): BspMesh {
	const positions = [
		// floor quad (z=0) verts 0..3
		0, 0, 0, 2, 0, 0, 2, 2, 0, 0, 2, 0,
		// ceiling quad (z=10) verts 4..7
		5, 0, 10, 7, 0, 10, 7, 2, 10, 5, 2, 10
	];
	const colors = [
		// floor red
		1, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0,
		// ceiling blue
		0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1
	];
	const indices = [
		0,
		1,
		2,
		0,
		2,
		3, // floor (tris 0,1)
		4,
		5,
		6,
		4,
		6,
		7 // ceiling (tris 2,3)
	];
	return {
		game: 'haloce',
		scenario: 'levels\\test\\twoquad\\twoquad',
		sourceMap: 'twoquad.map',
		bounds: { minX: 0, maxX: 7, minY: 0, maxY: 2, minZ: 0, maxZ: 10 },
		positions,
		colors,
		indices
	};
}

describe('buildTriMeta', () => {
	it('computes centroid, z-extent and averaged material colour per triangle', () => {
		const mesh = twoQuadMesh();
		const meta = buildTriMeta(mesh);
		expect(triCount(mesh)).toBe(4);
		expect(meta).toHaveLength(4);
		// floor tris sit at z=0, red
		expect(meta[0].zMin).toBe(0);
		expect(meta[0].zMax).toBe(0);
		expect(meta[0].r).toBeCloseTo(1);
		expect(meta[0].b).toBeCloseTo(0);
		// ceiling tris sit at z=10, blue
		expect(meta[2].cz).toBe(10);
		expect(meta[2].b).toBeCloseTo(1);
		expect(meta[2].r).toBeCloseTo(0);
	});

	it('falls back to neutral grey when the mesh carries no colours', () => {
		const mesh = twoQuadMesh();
		const meta = buildTriMeta({ positions: mesh.positions, indices: mesh.indices });
		expect(meta[0].r).toBeCloseTo(0.5);
	});
});

describe('connectedComponents', () => {
	it('groups each quad into one component despite duplicated vertices', () => {
		const comp = connectedComponents(twoQuadMesh());
		// floor tris share a component; ceiling tris share a different one
		expect(comp[0]).toBe(comp[1]);
		expect(comp[2]).toBe(comp[3]);
		expect(comp[0]).not.toBe(comp[2]);
	});

	it('welds spatially-coincident but separately-indexed vertices', () => {
		// Two triangles meeting at a shared edge but with their OWN vertex copies.
		const mesh: BspMesh = {
			game: 'haloce',
			scenario: 's',
			sourceMap: 'm',
			bounds: { minX: 0, maxX: 2, minY: 0, maxY: 1, minZ: 0, maxZ: 0 },
			positions: [
				0,
				0,
				0,
				1,
				0,
				0,
				0,
				1,
				0, // tri 0
				1,
				0,
				0,
				1,
				1,
				0,
				0,
				1,
				0 // tri 1 reuses (1,0,0) and (0,1,0) by VALUE
			],
			indices: [0, 1, 2, 3, 4, 5]
		};
		const comp = connectedComponents(mesh);
		expect(comp[0]).toBe(comp[1]);
	});
});

describe('selectAbovePlane', () => {
	const meta = buildTriMeta(twoQuadMesh());
	it('centroid mode removes triangles whose centroid is at/above the plane', () => {
		expect(selectAbovePlane(meta, 5, 'centroid')).toEqual([2, 3]);
		expect(selectAbovePlane(meta, 10, 'centroid')).toEqual([2, 3]);
		expect(selectAbovePlane(meta, 11, 'centroid')).toEqual([]);
	});
	it('whole mode requires the entire triangle above the plane', () => {
		expect(selectAbovePlane(meta, 0, 'whole')).toEqual([0, 1, 2, 3]);
		expect(selectAbovePlane(meta, 10, 'whole')).toEqual([2, 3]);
	});
});

describe('selectComponent', () => {
	it('returns every triangle sharing the clicked triangle component', () => {
		const comp = connectedComponents(twoQuadMesh());
		expect(selectComponent(comp, 2)).toEqual([2, 3]);
		expect(selectComponent(comp, 0)).toEqual([0, 1]);
	});
	it('is empty for an out-of-range triangle', () => {
		const comp = connectedComponents(twoQuadMesh());
		expect(selectComponent(comp, 99)).toEqual([]);
	});
});

describe('selectByMaterial', () => {
	const meta = buildTriMeta(twoQuadMesh());
	it('selects all triangles sharing the clicked material colour', () => {
		expect(selectByMaterial(meta, 2, 0.1)).toEqual([2, 3]); // blue ceiling
		expect(selectByMaterial(meta, 0, 0.1)).toEqual([0, 1]); // red floor
	});
});

describe('boxSelect', () => {
	const meta = buildTriMeta(twoQuadMesh());
	// Fake projector: identity on (cx, cy) → screen, ignoring z.
	const project = (cx: number, cy: number): [number, number] => [cx, cy];
	it('selects triangles whose centroid projects inside the rect', () => {
		// Floor centroids are near x≈0.6..1.3; ceiling near x≈5.6..6.3.
		const got = boxSelect(meta, project, { minX: 4, minY: -1, maxX: 8, maxY: 3 });
		expect(got).toEqual([2, 3]);
	});
	it('skips triangles the projector rejects (behind camera)', () => {
		const rejecting = () => null;
		expect(boxSelect(meta, rejecting, { minX: -100, minY: -100, maxX: 100, maxY: 100 })).toEqual(
			[]
		);
	});
	it('pointInRect is inclusive on the edges', () => {
		expect(pointInRect(4, 3, { minX: 4, minY: 3, maxX: 5, maxY: 6 })).toBe(true);
		expect(pointInRect(3.9, 3, { minX: 4, minY: 3, maxX: 5, maxY: 6 })).toBe(false);
	});
});

describe('exportSpectatorMesh', () => {
	it('keeps only non-removed triangles, re-indexes vertices, recomputes bounds', () => {
		const mesh = twoQuadMesh();
		const removed = new Uint8Array([0, 0, 1, 1]); // drop the ceiling
		const out = exportSpectatorMesh(mesh, removed, { cullZ: 5, sourceMesh: 'twoquad.json' });
		expect(out.kind).toBe('spectator-mesh');
		expect(out.triangle_count).toBe(2);
		// Floor quad has 4 unique verts → re-indexed to 0..3, no ceiling verts.
		expect(out.vertex_count).toBe(4);
		expect(out.positions).toHaveLength(12);
		expect(Math.max(...out.indices)).toBe(3);
		// Bounds collapse to the kept floor (z=0 only).
		expect(out.bounds.maxZ).toBe(0);
		expect(out.bounds.maxX).toBe(2);
		expect(out.colors).toHaveLength(12);
		expect(out.cull_z).toBe(5);
		expect(out.source_mesh).toBe('twoquad.json');
	});

	it('round-trips through normalizeMesh-compatible fields (re-import)', () => {
		const mesh = twoQuadMesh();
		const out = exportSpectatorMesh(mesh, new Uint8Array([0, 0, 0, 0]));
		// Same schema the loader expects: snake_case scenario/source_map + arrays.
		expect(out.scenario).toBe(mesh.scenario);
		expect(out.source_map).toBe(mesh.sourceMap);
		expect(out.positions.length % 3).toBe(0);
		expect(out.indices.length % 3).toBe(0);
		expect(out.triangle_count).toBe(4);
	});

	it('drops the colours array entirely when the source has none', () => {
		const mesh = twoQuadMesh();
		const noColor: BspMesh = { ...mesh, colors: undefined };
		const out = exportSpectatorMesh(noColor, new Uint8Array([0, 0, 0, 0]));
		expect(out.colors).toBeUndefined();
	});
});

describe('editStats', () => {
	it('tallies kept/removed triangles and kept vertices', () => {
		const mesh = twoQuadMesh();
		const s = editStats(mesh, new Uint8Array([0, 0, 1, 1]));
		expect(s.totalTris).toBe(4);
		expect(s.keptTris).toBe(2);
		expect(s.removedTris).toBe(2);
		expect(s.keptVerts).toBe(4);
	});
});

describe('defaultCullZ', () => {
	it('returns a height inside the mesh Z range', () => {
		const mesh = twoQuadMesh();
		const z = defaultCullZ(mesh);
		expect(z).toBeGreaterThanOrEqual(mesh.bounds.minZ + 0.1);
		expect(z).toBeLessThanOrEqual(mesh.bounds.maxZ - 0.05);
	});
});

describe('RemovedHistory', () => {
	it('commits, undoes and redoes removed-state snapshots', () => {
		const h = new RemovedHistory(new Uint8Array([0, 0, 0, 0]));
		expect(h.canUndo()).toBe(false);

		const s1 = h.current();
		s1[2] = 1;
		h.commit(s1);
		expect(h.canUndo()).toBe(true);

		const s2 = h.current();
		s2[3] = 1;
		h.commit(s2);
		expect(Array.from(h.current())).toEqual([0, 0, 1, 1]);

		const back = h.undo();
		expect(Array.from(back!)).toEqual([0, 0, 1, 0]);
		const back2 = h.undo();
		expect(Array.from(back2!)).toEqual([0, 0, 0, 0]);
		expect(h.undo()).toBeNull();

		const fwd = h.redo();
		expect(Array.from(fwd!)).toEqual([0, 0, 1, 0]);
	});

	it('clears the redo stack on a fresh commit', () => {
		const h = new RemovedHistory(new Uint8Array([0, 0]));
		const a = h.current();
		a[0] = 1;
		h.commit(a);
		h.undo();
		expect(h.canRedo()).toBe(true);
		const b = h.current();
		b[1] = 1;
		h.commit(b);
		expect(h.canRedo()).toBe(false);
		expect(Array.from(h.current())).toEqual([0, 1]);
	});

	it('snapshots are independent copies (no aliasing into history)', () => {
		const h = new RemovedHistory(new Uint8Array([0, 0]));
		const s = h.current();
		s[0] = 1;
		h.commit(s);
		s[1] = 1; // mutate AFTER commit — must not corrupt the stored snapshot
		expect(Array.from(h.current())).toEqual([1, 0]);
	});
});
