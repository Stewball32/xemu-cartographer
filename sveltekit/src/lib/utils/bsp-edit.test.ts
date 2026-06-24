import { describe, it, expect } from 'vitest';
import type { BspMesh } from './game-geometry';
import {
	triCount,
	buildTriMeta,
	connectedComponents,
	buildAdjacency,
	selectCoplanar,
	selectAbovePlane,
	selectBeyondPlane,
	selectComponent,
	selectByMaterial,
	selectByTag,
	boxSelect,
	pointInRect,
	autoClassify,
	semanticToTag,
	degenerateTriangleIndices,
	tagOf,
	tagCounts,
	tagSignature,
	buildTagOverrides,
	applyTagOverrides,
	taggedFloorMarkers,
	taggedFloorZs,
	defaultCullZ,
	isBaked,
	exportSpectatorMesh,
	editStats,
	defaultTagRender,
	buildTagSidecar,
	TAG_INDEX,
	EditHistory,
	type EditState
} from './bsp-edit';

/** Test scene exercising the WINDING-INDEPENDENT enclosure rule:
 *  - tris 0,1: floor quad at z=0 over XY [0..2]², with a DOWNWARD (-Z) normal
 *    (flipped winding) — the exact bug case. It's enclosed by the ceiling above,
 *    so it must still classify as floor.
 *  - tris 2,3: ceiling quad at z=4 over the SAME XY [0..2]² (topmost → ceiling).
 *  - tri 4:   a vertical wall at x=5 (normal ±X).
 *  - tri 5:   a 45° up-facing ramp near (8,4).
 *  Vertices are duplicated per face so welding-based adjacency is exercised. */
function scene(): BspMesh {
	const positions = [
		// floor (z=0), CW-from-above winding → normal points DOWN (-Z); verts 0..3
		0, 0, 0, 0, 2, 0, 2, 2, 0, 2, 0, 0,
		// ceiling (z=4) over the same footprint; verts 4..7
		0, 0, 4, 2, 0, 4, 2, 2, 4, 0, 2, 4,
		// wall: vertical at x=5; verts 8..10
		5, 0, 0, 5, 2, 0, 5, 0, 2,
		// ramp: up-facing 45° slope; verts 11..13
		8, 3, 0, 9, 3, 1, 8, 5, 0
	];
	const colors = [
		1,
		0,
		0,
		1,
		0,
		0,
		1,
		0,
		0,
		1,
		0,
		0, // floor red
		0,
		0,
		1,
		0,
		0,
		1,
		0,
		0,
		1,
		0,
		0,
		1, // ceiling blue
		0.5,
		0.5,
		0.5,
		0.5,
		0.5,
		0.5,
		0.5,
		0.5,
		0.5, // wall grey
		0.2,
		0.8,
		0.2,
		0.2,
		0.8,
		0.2,
		0.2,
		0.8,
		0.2 // ramp green
	];
	const indices = [0, 1, 2, 0, 2, 3, 4, 5, 6, 4, 6, 7, 8, 9, 10, 11, 12, 13];
	return {
		game: 'haloce',
		scenario: 'levels\\test\\scene\\scene',
		sourceMap: 'scene.map',
		bounds: { minX: 0, maxX: 9, minY: 0, maxY: 5, minZ: 0, maxZ: 4 },
		positions,
		colors,
		indices
	};
}

describe('buildTriMeta', () => {
	it('computes centroid, z-extent, normal, XY bbox and material colour', () => {
		const mesh = scene();
		const meta = buildTriMeta(mesh);
		expect(triCount(mesh)).toBe(6);
		expect(meta[0].nz).toBeLessThan(0); // floor has a DOWNWARD normal here
		expect(Math.abs(meta[0].nz)).toBeCloseTo(1);
		expect(meta[0].minX).toBe(0);
		expect(meta[0].maxX).toBe(2);
		expect(Math.abs(meta[4].nx)).toBeCloseTo(1); // wall normal ~±X
	});
});

describe('autoClassify (enclosure, winding-independent)', () => {
	it('classifies a downward-normal floor as FLOOR because it is enclosed', () => {
		const meta = buildTriMeta(scene());
		const tags = autoClassify(meta);
		expect(tagOf(tags, 0)).toBe('floor'); // ← the bug case: -Z normal, still floor
		expect(tagOf(tags, 1)).toBe('floor');
	});
	it('classifies the topmost horizontal surface as CEILING', () => {
		const tags = autoClassify(buildTriMeta(scene()));
		expect(tagOf(tags, 2)).toBe('ceiling');
		expect(tagOf(tags, 3)).toBe('ceiling');
	});
	it('classifies vertical as wall and sloped as ramp', () => {
		const tags = autoClassify(buildTriMeta(scene()));
		expect(tagOf(tags, 4)).toBe('wall');
		expect(tagOf(tags, 5)).toBe('ramp');
	});
	it('never auto-assigns inaccessible/clutter', () => {
		const c = tagCounts(autoClassify(buildTriMeta(scene())));
		expect(c.inaccessible).toBe(0);
		expect(c.clutter).toBe(0);
	});
});

describe('material-name priors', () => {
	/** scene() with per-material semantics attached:
	 *  - mat 0 "floor", mat 1 "ceiling" (a light), mat 2 "wall".
	 *  tris 0,1 (enclosed floor) → light; 2,3 (topmost) → floor; 4 (wall) → floor;
	 *  5 (ramp) → wall. Exercises every override + every defer-to-geometry case. */
	function taggedScene(): BspMesh {
		return {
			...scene(),
			materials: [
				'lvl\\shaders\\plate floor',
				'lvl\\shaders\\overhead light',
				'lvl\\shaders\\metal'
			],
			materialSemantic: ['floor', 'ceiling', 'wall'],
			triangleMaterial: [1, 1, 0, 0, 0, 2]
		};
	}

	it('semanticToTag maps classes to tag priors (or null)', () => {
		expect(semanticToTag('floor')).toBe('floor');
		expect(semanticToTag('ramp')).toBe('ramp');
		expect(semanticToTag('grate')).toBe('floor'); // walkable grating
		expect(semanticToTag('glass')).toBe('wall'); // see-through boundary
		expect(semanticToTag('sky')).toBe('ceiling'); // overhead → culled
		expect(semanticToTag('ceiling')).toBe('ceiling');
		expect(semanticToTag('ladder')).toBeNull(); // geometry decides
		expect(semanticToTag('other')).toBeNull();
		expect(semanticToTag('')).toBeNull();
		expect(semanticToTag(undefined)).toBeNull();
	});

	it('buildTriMeta attaches a prior from the material name, null without one', () => {
		expect(buildTriMeta(scene())[0].prior).toBeNull(); // no material data
		const meta = buildTriMeta(taggedScene());
		expect(meta[0].prior).toBe('ceiling'); // mat 1 (light)
		expect(meta[2].prior).toBe('floor'); // mat 0
		expect(meta[5].prior).toBe('wall'); // mat 2
	});

	it('a floor material flips a geometry-CEILING (topmost) surface to floor', () => {
		const tags = autoClassify(buildTriMeta(taggedScene()));
		// tris 2,3 are the topmost horizontal surface → geometry calls them ceiling,
		// but the floor material overrides → floor (the headline fix).
		expect(tagOf(tags, 2)).toBe('floor');
		expect(tagOf(tags, 3)).toBe('floor');
	});

	it('an overhead-light material culls a geometry-FLOOR surface to ceiling', () => {
		const tags = autoClassify(buildTriMeta(taggedScene()));
		// tris 0,1 are an enclosed floor → geometry floor, but the light material
		// reclassifies them to ceiling (dropped from the bake) regardless of facing.
		expect(tagOf(tags, 0)).toBe('ceiling');
		expect(tagOf(tags, 1)).toBe('ceiling');
	});

	it('a floor material on a vertical surface stays wall (geometry wins)', () => {
		const tags = autoClassify(buildTriMeta(taggedScene()));
		expect(tagOf(tags, 4)).toBe('wall'); // vertical trim, not a floor
	});

	it('a wall material never overrides a genuine ramp', () => {
		const tags = autoClassify(buildTriMeta(taggedScene()));
		expect(tagOf(tags, 5)).toBe('ramp'); // sloped deck preserved
	});

	it('flags degenerate (collinear, zero-area) triangles as stray-line artifacts', () => {
		// scene()'s 6 real tris carry finite area; append a collinear triangle.
		const base = scene();
		const mesh: BspMesh = {
			...base,
			positions: [...base.positions, 0, 0, 6, 1, 0, 6, 2, 0, 6], // 3 collinear verts
			indices: [...base.indices, 14, 15, 16]
		};
		const meta = buildTriMeta(mesh);
		expect(meta[6].area).toBeCloseTo(0); // the collinear triangle
		expect(meta[0].area).toBeGreaterThan(0.1); // a real floor quad-half
		expect(degenerateTriangleIndices(meta)).toEqual([6]);
		// real geometry is never flagged on its own.
		expect(degenerateTriangleIndices(buildTriMeta(base))).toEqual([]);
	});

	it('priors leave a clean mesh unchanged vs geometry where they agree', () => {
		// scene() with no material data classifies identically to the prior path
		// (no priors → geometry alone): the no-regression guarantee.
		const geom = autoClassify(buildTriMeta(scene()));
		expect(tagOf(geom, 0)).toBe('floor');
		expect(tagOf(geom, 2)).toBe('ceiling');
		expect(tagOf(geom, 4)).toBe('wall');
		expect(tagOf(geom, 5)).toBe('ramp');
	});
});

describe('stable tag persistence', () => {
	it('signature is stable for the same surface across a winding flip', () => {
		const meta = buildTriMeta(scene());
		// Same plane + centroid as floor-tri 0 but wound the OTHER way (flipped
		// normal) — re-extract often flips winding; the key must survive it.
		const flipped = buildTriMeta({
			...scene(),
			positions: [0, 0, 0, 2, 2, 0, 0, 2, 0],
			indices: [0, 1, 2]
		});
		expect(flipped[0].nz).toBeGreaterThan(0); // opposite sign to meta[0]
		expect(tagSignature(flipped[0])).toBe(tagSignature(meta[0]));
	});
	it('builds overrides only for hand-edited tags and re-applies them', () => {
		const meta = buildTriMeta(scene());
		const auto = autoClassify(meta);
		const edited = Uint8Array.from(auto);
		edited[2] = TAG_INDEX.inaccessible; // retag the ceiling
		const overrides = buildTagOverrides(meta, edited, auto);
		expect(overrides).toHaveLength(1);
		expect(overrides[0].tag).toBe('inaccessible');
		const reapplied = applyTagOverrides(meta, auto, overrides);
		expect(tagOf(reapplied, 2)).toBe('inaccessible');
		expect(tagOf(reapplied, 0)).toBe('floor');
	});
});

describe('selectByTag / connectedComponents', () => {
	it('selects all triangles carrying a tag, groups welded quads', () => {
		const mesh = scene();
		const tags = autoClassify(buildTriMeta(mesh));
		expect(selectByTag(tags, 'floor')).toEqual([0, 1]);
		expect(selectByTag(tags, 'ceiling')).toEqual([2, 3]);
		const comp = connectedComponents(mesh);
		expect(comp[0]).toBe(comp[1]);
		expect(comp[0]).not.toBe(comp[2]);
	});
});

describe('selectCoplanar', () => {
	it('selects the whole flat face via adjacency + coplanarity', () => {
		const mesh = scene();
		const meta = buildTriMeta(mesh);
		const adj = buildAdjacency(mesh);
		// Clicking one floor triangle grabs the whole floor quad, not the ceiling.
		expect(selectCoplanar(adj, meta, 0, 8, 0.2)).toEqual([0, 1]);
		// The ceiling is a different parallel plane → not included.
		expect(selectCoplanar(adj, meta, 2, 8, 0.2)).toEqual([2, 3]);
	});
	it('does not cross to a non-coplanar neighbour', () => {
		const mesh = scene();
		const sel = selectCoplanar(buildAdjacency(mesh), buildTriMeta(mesh), 4, 8, 0.2);
		expect(sel).toEqual([4]); // lone wall triangle
	});
});

describe('selectBeyondPlane (multi-axis + flip)', () => {
	const meta = buildTriMeta(scene());
	it('Z+ above the plane (the classic roof cut)', () => {
		expect(selectAbovePlane(meta, 2, 'centroid')).toEqual([2, 3]); // ceiling at z=4
	});
	it('X axis, positive side clips the right-hand geometry', () => {
		// wall (cx=5) + ramp (cx≈8.3) are beyond x=4; floor/ceiling (cx≈1) are not.
		expect(selectBeyondPlane(meta, 'x', 4, 1, 'centroid')).toEqual([4, 5]);
	});
	it('X axis, negative side flips to the left-hand geometry', () => {
		expect(selectBeyondPlane(meta, 'x', 4, -1, 'centroid')).toEqual([0, 1, 2, 3]);
	});
});

describe('selectComponent / selectByMaterial / boxSelect', () => {
	const mesh = scene();
	const meta = buildTriMeta(mesh);
	it('component + material selection', () => {
		expect(selectComponent(connectedComponents(mesh), 2)).toEqual([2, 3]);
		expect(selectByMaterial(meta, 0, 0.1)).toEqual([0, 1]); // red floor
	});
	it('box select by projected centroid', () => {
		const project = (cx: number): [number, number] => [cx, 0];
		expect(boxSelect(meta, project, { minX: 4, minY: -1, maxX: 6, maxY: 1 })).toEqual([4]); // wall x=5
	});
	it('pointInRect inclusive on edges', () => {
		expect(pointInRect(4, 3, { minX: 4, minY: 3, maxX: 5, maxY: 6 })).toBe(true);
		expect(pointInRect(3.9, 3, { minX: 4, minY: 3, maxX: 5, maxY: 6 })).toBe(false);
	});
});

describe('tag-driven floors + cull default', () => {
	it('floor markers come from floor + ramp tags only', () => {
		const meta = buildTriMeta(scene());
		const tags = autoClassify(meta);
		expect(taggedFloorMarkers(meta, tags)).toHaveLength(3); // 2 floor + 1 ramp
		expect(taggedFloorZs(meta, tags).every((z) => z < 1.1)).toBe(true);
	});
	it('default cull height sits above the floors, inside the Z range', () => {
		const mesh = scene();
		const meta = buildTriMeta(mesh);
		const z = defaultCullZ(mesh, meta, autoClassify(meta));
		expect(z).toBeGreaterThan(0.5);
		expect(z).toBeLessThanOrEqual(mesh.bounds.maxZ - 0.05);
	});
});

describe('isBaked + export', () => {
	it('default render rules keep floor/ramp/wall and drop ceiling', () => {
		const meta = buildTriMeta(scene());
		const tags = autoClassify(meta);
		const removed = new Uint8Array(meta.length);
		const render = defaultTagRender();
		expect(isBaked(0, removed, tags, render)).toBe(true); // floor (flipped normal)
		expect(isBaked(4, removed, tags, render)).toBe(true); // wall
		expect(isBaked(2, removed, tags, render)).toBe(false); // ceiling dropped
	});

	it('bakes only kept triangles, embeds tags, recomputes bounds', () => {
		const mesh = scene();
		const meta = buildTriMeta(mesh);
		const auto = autoClassify(meta);
		const tags = Uint8Array.from(auto);
		const removed = new Uint8Array(meta.length);
		const render = defaultTagRender();
		const out = exportSpectatorMesh(
			mesh,
			{ removed, tags },
			{ meta, autoTags: auto, tagRender: render, cullZ: 2, sourceMesh: 'scene.json' }
		);
		expect(out.schema_version).toBe(2);
		// floor(2) + wall(1) + ramp(1) kept; ceiling(2) dropped → 4 tris.
		expect(out.triangle_count).toBe(4);
		expect(out.tags).toHaveLength(4);
		// Ceiling at z=4 dropped → kept geometry tops out at the wall (z=2).
		expect(out.bounds.maxZ).toBe(2);
		expect(out.tag_render.ceiling).toBe(false);
		expect(out.tag_overrides).toEqual([]);
	});

	it('a manual delete + a render toggle both drop geometry from the bake', () => {
		const mesh = scene();
		const meta = buildTriMeta(mesh);
		const auto = autoClassify(meta);
		const removed = new Uint8Array(meta.length);
		removed[0] = 1; // delete one floor tri
		const render = defaultTagRender();
		render.wall = false;
		const out = exportSpectatorMesh(
			mesh,
			{ removed, tags: Uint8Array.from(auto) },
			{ meta, autoTags: auto, tagRender: render }
		);
		// floor 2-1=1; wall off=0; ramp 1; ceiling off=0 → 2 tris.
		expect(out.triangle_count).toBe(2);
	});
});

describe('buildTagSidecar', () => {
	it('captures legend, render map and only the hand-edited overrides', () => {
		const meta = buildTriMeta(scene());
		const auto = autoClassify(meta);
		const tags = Uint8Array.from(auto);
		tags[4] = TAG_INDEX.inaccessible; // retag the wall
		const sidecar = buildTagSidecar(meta, tags, auto, defaultTagRender());
		expect(sidecar.kind).toBe('spectator-tags');
		expect(sidecar.overrides).toHaveLength(1);
		expect(sidecar.overrides[0].tag).toBe('inaccessible');
		expect(sidecar.render.floor).toBe(true);
	});
});

describe('editStats', () => {
	it('tallies baked vs dropped under the render rules', () => {
		const mesh = scene();
		const meta = buildTriMeta(mesh);
		const tags = autoClassify(meta);
		const s = editStats(mesh, new Uint8Array(meta.length), tags, defaultTagRender());
		expect(s.totalTris).toBe(6);
		expect(s.keptTris).toBe(4); // ceiling(2) dropped
		expect(s.removedTris).toBe(2);
	});
});

describe('EditHistory', () => {
	const blank = (): EditState => ({ removed: new Uint8Array(3), tags: new Uint8Array(3) });

	it('commits, undoes and redoes {removed,tags} snapshots', () => {
		const h = new EditHistory(blank());
		expect(h.canUndo()).toBe(false);
		const s1 = h.current();
		s1.removed[1] = 1;
		h.commit(s1);
		const s2 = h.current();
		s2.tags[2] = TAG_INDEX.wall;
		h.commit(s2);
		expect(Array.from(h.current().tags)).toEqual([0, 0, TAG_INDEX.wall]);
		const back = h.undo();
		expect(Array.from(back!.removed)).toEqual([0, 1, 0]);
		expect(Array.from(back!.tags)).toEqual([0, 0, 0]);
		h.undo();
		expect(h.canUndo()).toBe(false);
		expect(Array.from(h.redo()!.removed)).toEqual([0, 1, 0]);
	});

	it('snapshots are independent copies', () => {
		const h = new EditHistory(blank());
		const s = h.current();
		s.removed[0] = 1;
		h.commit(s);
		s.removed[1] = 1;
		expect(Array.from(h.current().removed)).toEqual([1, 0, 0]);
	});
});
