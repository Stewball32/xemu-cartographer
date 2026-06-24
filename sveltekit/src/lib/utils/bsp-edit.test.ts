import { describe, it, expect } from 'vitest';
import type { BspMesh } from './game-geometry';
import {
	triCount,
	buildTriMeta,
	connectedComponents,
	selectAbovePlane,
	selectComponent,
	selectByMaterial,
	selectByTag,
	boxSelect,
	pointInRect,
	autoClassify,
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

/** A tiny test scene with one of each orientation, vertices duplicated per face:
 *  - tris 0,1: floor quad at z=0 (normal +Z), red, XY [0..2]²
 *  - tris 2,3: ceiling quad at z=10 (normal -Z), blue, XY [5..7]×[0..2]
 *  - tri 4:   a vertical wall (normal ~+X) at x≈3
 *  - tri 5:   a 45° ramp (up-facing, sloped) climbing from z=0 to z=1 */
function scene(): BspMesh {
	const positions = [
		// floor (z=0) verts 0..3, CCW from above → normal +Z
		0, 0, 0, 2, 0, 0, 2, 2, 0, 0, 2, 0,
		// ceiling (z=10) verts 4..7, CW from above → normal -Z
		5, 0, 10, 5, 2, 10, 7, 2, 10, 7, 0, 10,
		// wall: vertical triangle in the Y-Z plane at x=3 → normal ±X (verts 8..10)
		3, 0, 0, 3, 2, 0, 3, 0, 2,
		// ramp: up-facing, sloped 45° (rises in +X) → normal tilts (verts 11..13)
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
		bounds: { minX: 0, maxX: 9, minY: 0, maxY: 5, minZ: 0, maxZ: 10 },
		positions,
		colors,
		indices
	};
}

describe('buildTriMeta', () => {
	it('computes centroid, z-extent, normal and material colour per triangle', () => {
		const mesh = scene();
		const meta = buildTriMeta(mesh);
		expect(triCount(mesh)).toBe(6);
		expect(Math.abs(meta[0].nz)).toBeCloseTo(1); // floor normal points along Z
		expect(meta[0].r).toBeCloseTo(1);
		expect(Math.abs(meta[4].nz)).toBeLessThan(0.2); // wall normal ~horizontal
		expect(Math.abs(meta[4].nx)).toBeCloseTo(1);
	});
});

describe('autoClassify', () => {
	it('seeds floor / ceiling / wall / ramp from the face normal', () => {
		const meta = buildTriMeta(scene());
		const tags = autoClassify(meta);
		expect(tagOf(tags, 0)).toBe('floor');
		expect(tagOf(tags, 1)).toBe('floor');
		expect(tagOf(tags, 2)).toBe('ceiling');
		expect(tagOf(tags, 4)).toBe('wall');
		expect(tagOf(tags, 5)).toBe('ramp'); // up-facing 45° slope
	});
	it('never auto-assigns inaccessible/clutter', () => {
		const tags = autoClassify(buildTriMeta(scene()));
		const counts = tagCounts(tags);
		expect(counts.inaccessible).toBe(0);
		expect(counts.clutter).toBe(0);
	});
});

describe('stable tag persistence', () => {
	it('signature survives a vertex reorder of the same triangle', () => {
		const meta = buildTriMeta(scene());
		// Same floor triangle, vertices listed in a rotated order → same plane/centroid.
		const rotated: BspMesh = {
			...scene(),
			positions: [2, 0, 0, 2, 2, 0, 0, 0, 0],
			indices: [0, 1, 2]
		};
		const m2 = buildTriMeta(rotated);
		// Centroid + dominant axis match → same coarse signature.
		expect(tagSignature(m2[0])).toBe(tagSignature(meta[0]));
	});

	it('builds overrides only for hand-edited tags and re-applies them by signature', () => {
		const meta = buildTriMeta(scene());
		const auto = autoClassify(meta);
		const edited = Uint8Array.from(auto);
		edited[2] = TAG_INDEX.inaccessible; // retag the ceiling as inaccessible
		const overrides = buildTagOverrides(meta, edited, auto);
		expect(overrides).toHaveLength(1);
		expect(overrides[0].tag).toBe('inaccessible');

		// Re-applying onto a fresh auto-classify restores the manual fix.
		const reapplied = applyTagOverrides(meta, auto, overrides);
		expect(tagOf(reapplied, 2)).toBe('inaccessible');
		expect(tagOf(reapplied, 0)).toBe('floor'); // untouched
	});
});

describe('selectByTag', () => {
	it('selects all triangles carrying a tag', () => {
		const tags = autoClassify(buildTriMeta(scene()));
		expect(selectByTag(tags, 'floor')).toEqual([0, 1]);
		expect(selectByTag(tags, 'ceiling')).toEqual([2, 3]);
		expect(selectByTag(tags, 'wall')).toEqual([4]);
	});
});

describe('connectedComponents', () => {
	it('groups each welded quad into one component', () => {
		const comp = connectedComponents(scene());
		expect(comp[0]).toBe(comp[1]);
		expect(comp[2]).toBe(comp[3]);
		expect(comp[0]).not.toBe(comp[2]);
	});
});

describe('selectAbovePlane', () => {
	const meta = buildTriMeta(scene());
	it('centroid mode removes triangles whose centroid is at/above the plane', () => {
		expect(selectAbovePlane(meta, 5, 'centroid')).toEqual([2, 3]); // only the z=10 ceiling
	});
});

describe('selectComponent / selectByMaterial', () => {
	it('selects the clicked connected piece and the clicked material', () => {
		const mesh = scene();
		const comp = connectedComponents(mesh);
		const meta = buildTriMeta(mesh);
		expect(selectComponent(comp, 2)).toEqual([2, 3]);
		expect(selectByMaterial(meta, 0, 0.1)).toEqual([0, 1]); // red floor
	});
});

describe('boxSelect', () => {
	const meta = buildTriMeta(scene());
	const project = (cx: number, cy: number): [number, number] => [cx, cy];
	it('selects triangles whose centroid projects inside the rect', () => {
		expect(boxSelect(meta, project, { minX: 4, minY: -1, maxX: 8, maxY: 3 })).toEqual([2, 3]);
	});
	it('pointInRect is inclusive on the edges', () => {
		expect(pointInRect(4, 3, { minX: 4, minY: 3, maxX: 5, maxY: 6 })).toBe(true);
		expect(pointInRect(3.9, 3, { minX: 4, minY: 3, maxX: 5, maxY: 6 })).toBe(false);
	});
});

describe('tag-driven floors + cull default', () => {
	it('floor markers/Zs come from floor + ramp tags only', () => {
		const meta = buildTriMeta(scene());
		const tags = autoClassify(meta);
		const markers = taggedFloorMarkers(meta, tags);
		// 2 floor tris + 1 ramp tri = 3 markers; no walls/ceilings.
		expect(markers).toHaveLength(3);
		expect(taggedFloorZs(meta, tags).every((z) => z < 1.1)).toBe(true);
	});
	it('default cull height sits above the floors but inside the Z range', () => {
		const mesh = scene();
		const meta = buildTriMeta(mesh);
		const tags = autoClassify(meta);
		const z = defaultCullZ(mesh, meta, tags);
		expect(z).toBeGreaterThan(0.5);
		expect(z).toBeLessThanOrEqual(mesh.bounds.maxZ - 0.05);
	});
});

describe('isBaked + export', () => {
	it('default render rules keep floor/ramp/wall and drop ceiling', () => {
		const mesh = scene();
		const meta = buildTriMeta(mesh);
		const tags = autoClassify(meta);
		const removed = new Uint8Array(meta.length);
		const render = defaultTagRender();
		expect(isBaked(0, removed, tags, render)).toBe(true); // floor
		expect(isBaked(4, removed, tags, render)).toBe(true); // wall
		expect(isBaked(2, removed, tags, render)).toBe(false); // ceiling dropped
	});

	it('bakes only kept triangles, embeds tags + overrides, recomputes bounds', () => {
		const mesh = scene();
		const meta = buildTriMeta(mesh);
		const auto = autoClassify(meta);
		const tags = Uint8Array.from(auto);
		const removed = new Uint8Array(meta.length);
		const render = defaultTagRender();
		const state: EditState = { removed, tags };
		const out = exportSpectatorMesh(mesh, state, {
			meta,
			autoTags: auto,
			tagRender: render,
			cullZ: 5,
			sourceMesh: 'scene.json'
		});
		expect(out.schema_version).toBe(2);
		expect(out.kind).toBe('spectator-mesh');
		// floor(2) + wall(1) + ramp(1) kept; ceiling(2) dropped → 4 tris.
		expect(out.triangle_count).toBe(4);
		expect(out.tags).toHaveLength(4);
		expect(out.tag_legend[out.tags[0]]).toBe('floor');
		// Ceiling at z=10 dropped → bounds collapse to the kept wall top (z=2).
		expect(out.bounds.maxZ).toBe(2);
		expect(out.tag_render.ceiling).toBe(false);
		expect(out.tag_overrides).toEqual([]); // nothing hand-edited
	});

	it('a manual delete + a render toggle both drop geometry from the bake', () => {
		const mesh = scene();
		const meta = buildTriMeta(mesh);
		const auto = autoClassify(meta);
		const tags = Uint8Array.from(auto);
		const removed = new Uint8Array(meta.length);
		removed[0] = 1; // manually delete one floor tri
		const render = defaultTagRender();
		render.wall = false; // turn walls off for this bake
		const out = exportSpectatorMesh(
			mesh,
			{ removed, tags },
			{ meta, autoTags: auto, tagRender: render }
		);
		// floor: 2 - 1 deleted = 1; wall off = 0; ramp 1; ceiling off = 0 → 2 tris.
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
	it('tallies baked vs dropped triangles under the render rules', () => {
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
		expect(Array.from(back!.tags)).toEqual([0, 0, 0]);
		expect(Array.from(back!.removed)).toEqual([0, 1, 0]);
		h.undo();
		expect(h.canUndo()).toBe(false);

		const fwd = h.redo();
		expect(Array.from(fwd!.removed)).toEqual([0, 1, 0]);
	});

	it('snapshots are independent copies (no aliasing into history)', () => {
		const h = new EditHistory(blank());
		const s = h.current();
		s.removed[0] = 1;
		h.commit(s);
		s.removed[1] = 1; // mutate AFTER commit — must not corrupt the stored snapshot
		expect(Array.from(h.current().removed)).toEqual([1, 0, 0]);
	});
});
