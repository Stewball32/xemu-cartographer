// Pure editing model for the BSP spectator-mesh editor (/bsp-editor).
//
// The editor loads the SAME structure-BSP mesh the visualizers consume
// ($lib/utils/game-geometry's BspMesh: flat positions/indices/colors in Halo
// world coords) and lets the operator cull it down to a clean "spectator mesh".
//
// SURFACE TAGGING is the primary mechanism. Every triangle carries a semantic
// tag (floor / ramp / wall / ceiling / inaccessible / clutter); an auto-classify
// pass seeds tags from the face normal, the operator fixes mistags by
// selection, and a per-tag render rule decides what survives the bake. The
// cull-height plane + manual delete stay as scalpels for stragglers. Tags drive
// the elevation banding too (floors/ramps are what get banded).
//
// Tags can't live in the BSP/.map, so they persist in OUR derived asset: the
// baked spectator mesh embeds per-kept-triangle tags + the legend + the render
// map, AND a sidecar of MANUAL overrides keyed by a STABLE spatial signature
// (quantised centroid + dominant normal axis) so a later re-extract/re-bake
// re-applies the operator's hand-edits instead of wiping them.
//
// Everything here is PURE / IO-free / DOM-free so the parts that must be correct
// — classification, connectivity, selection, stable keying, vertex re-indexing on
// export, undo/redo — unit-test without a browser or a GPU. EditorScene.svelte
// does only the Three.js wiring on top of these.
//
// A triangle is the unit of editing, indexed by ordinal t = 0..triCount-1;
// triangle t spans mesh.indices[3t..3t+2].

import type { BspMesh } from '$lib/utils/game-geometry';

// --- Surface taxonomy -------------------------------------------------------

export type SurfaceTag = 'floor' | 'ramp' | 'wall' | 'ceiling' | 'inaccessible' | 'clutter';

/** Stable, ordered legend — the index stored per triangle is an index INTO this. */
export const TAG_LEGEND: SurfaceTag[] = [
	'floor',
	'ramp',
	'wall',
	'ceiling',
	'inaccessible',
	'clutter'
];

export interface TagDef {
	tag: SurfaceTag;
	label: string;
	/** Editor highlight colour (0..1 RGB). */
	color: [number, number, number];
	/** Whether this tag's geometry survives the bake by default. floors/ramps/
	 *  walls render; ceiling/inaccessible/clutter are dropped unless toggled on. */
	defaultRender: boolean;
}

export const TAG_DEFS: TagDef[] = [
	{ tag: 'floor', label: 'Floor', color: [0.3, 0.74, 0.55], defaultRender: true },
	{ tag: 'ramp', label: 'Ramp / stairs', color: [0.67, 0.8, 0.32], defaultRender: true },
	{ tag: 'wall', label: 'Wall', color: [0.46, 0.56, 0.82], defaultRender: true },
	{ tag: 'ceiling', label: 'Ceiling / roof', color: [0.88, 0.45, 0.32], defaultRender: false },
	{ tag: 'inaccessible', label: 'Inaccessible', color: [0.55, 0.3, 0.34], defaultRender: false },
	{ tag: 'clutter', label: 'Clutter', color: [0.64, 0.46, 0.8], defaultRender: false }
];

export const TAG_INDEX: Record<SurfaceTag, number> = Object.fromEntries(
	TAG_LEGEND.map((t, i) => [t, i])
) as Record<SurfaceTag, number>;

export type TagRenderMap = Record<SurfaceTag, boolean>;

export function defaultTagRender(): TagRenderMap {
	const m = {} as TagRenderMap;
	for (const d of TAG_DEFS) m[d.tag] = d.defaultRender;
	return m;
}

/** Auto-classify thresholds on the normalized up-component (nz). */
const FLOOR_COS = 0.866; // ≤30° from horizontal → floor
const RAMP_COS = 0.5; // 30°–60° up-facing → ramp/stairs
const CEIL_COS = -0.5; // down-facing → ceiling

// --- Per-triangle metadata --------------------------------------------------

/** Per-triangle precomputed geometry + material, derived once from the mesh. */
export interface TriMeta {
	/** Centroid in Halo world coords. */
	cx: number;
	cy: number;
	cz: number;
	/** Corner Z extent (plane cull + band tagging read these). */
	zMin: number;
	zMax: number;
	/** Unit face normal in Halo world coords (Z up). */
	nx: number;
	ny: number;
	nz: number;
	/** Averaged vertex material colour (0..1); neutral grey when the mesh has none. */
	r: number;
	g: number;
	b: number;
}

const NEUTRAL_RGB: [number, number, number] = [0.5, 0.5, 0.52];

/** Number of triangles in the mesh. */
export function triCount(mesh: Pick<BspMesh, 'indices'>): number {
	return Math.floor(mesh.indices.length / 3);
}

/** Build per-triangle centroid / Z-extent / normal / material colour. */
export function buildTriMeta(mesh: Pick<BspMesh, 'positions' | 'indices' | 'colors'>): TriMeta[] {
	const pos = mesh.positions;
	const idx = mesh.indices;
	const col = mesh.colors && mesh.colors.length === pos.length ? mesh.colors : undefined;
	const T = triCount(mesh);
	const out: TriMeta[] = new Array(T);
	for (let t = 0; t < T; t++) {
		const a = idx[t * 3] * 3;
		const b = idx[t * 3 + 1] * 3;
		const c = idx[t * 3 + 2] * 3;
		const ax = pos[a],
			ay = pos[a + 1],
			az = pos[a + 2];
		const bx = pos[b],
			by = pos[b + 1],
			bz = pos[b + 2];
		const cx = pos[c],
			cy = pos[c + 1],
			cz = pos[c + 2];

		// Face normal = (b-a) × (c-a), normalized.
		let nx = (by - ay) * (cz - az) - (bz - az) * (cy - ay);
		let ny = (bz - az) * (cx - ax) - (bx - ax) * (cz - az);
		let nz = (bx - ax) * (cy - ay) - (by - ay) * (cx - ax);
		const len = Math.hypot(nx, ny, nz);
		if (len > 1e-9) {
			nx /= len;
			ny /= len;
			nz /= len;
		} else {
			nx = 0;
			ny = 0;
			nz = 1;
		}

		let r = NEUTRAL_RGB[0],
			g = NEUTRAL_RGB[1],
			bb = NEUTRAL_RGB[2];
		if (col) {
			r = (col[a] + col[b] + col[c]) / 3;
			g = (col[a + 1] + col[b + 1] + col[c + 1]) / 3;
			bb = (col[a + 2] + col[b + 2] + col[c + 2]) / 3;
			if (!Number.isFinite(r)) {
				r = NEUTRAL_RGB[0];
				g = NEUTRAL_RGB[1];
				bb = NEUTRAL_RGB[2];
			}
		}
		out[t] = {
			cx: (ax + bx + cx) / 3,
			cy: (ay + by + cy) / 3,
			cz: (az + bz + cz) / 3,
			zMin: Math.min(az, bz, cz),
			zMax: Math.max(az, bz, cz),
			nx,
			ny,
			nz,
			r,
			g,
			b: bb
		};
	}
	return out;
}

// --- Auto-classification ----------------------------------------------------

/**
 * First-pass tag per triangle from its face normal:
 *   up-facing & near-flat → floor; up-facing & sloped → ramp; near-vertical →
 *   wall; down-facing → ceiling. inaccessible/clutter are never auto-assigned —
 *   they're manual bins. The normal's sign is ambiguous on a double-sided BSP
 *   surface, so we read the ABSOLUTE up-component for the floor/ramp/wall split
 *   and only call a clearly down-facing surface a ceiling.
 */
export function autoClassify(meta: TriMeta[]): Uint8Array {
	const tags = new Uint8Array(meta.length);
	for (let t = 0; t < meta.length; t++) {
		const nz = meta[t].nz;
		const up = Math.abs(nz);
		let tag: SurfaceTag;
		if (nz <= CEIL_COS) tag = 'ceiling';
		else if (up >= FLOOR_COS) tag = 'floor';
		else if (up >= RAMP_COS) tag = nz > 0 ? 'ramp' : 'ceiling';
		else tag = 'wall';
		tags[t] = TAG_INDEX[tag];
	}
	return tags;
}

export function tagOf(tags: ArrayLike<number>, t: number): SurfaceTag {
	return TAG_LEGEND[tags[t]] ?? 'wall';
}

/** Count of triangles per tag (for the editor chips). */
export function tagCounts(tags: ArrayLike<number>): Record<SurfaceTag, number> {
	const counts = {} as Record<SurfaceTag, number>;
	for (const t of TAG_LEGEND) counts[t] = 0;
	for (let i = 0; i < tags.length; i++) counts[TAG_LEGEND[tags[i]] ?? 'wall']++;
	return counts;
}

// --- Stable tag persistence (survives re-extraction) ------------------------

/**
 * Stable spatial signature for a triangle: quantised centroid + dominant normal
 * axis. Survives minor retessellation/reordering across a geometry re-extract, so
 * the operator's MANUAL tag fixes can be re-applied to freshly-extracted geometry
 * instead of being wiped. Coarse by design (collisions are acceptable for a
 * best-effort re-apply).
 */
export function tagSignature(m: TriMeta, quantum = 0.5): string {
	const q = quantum > 0 ? quantum : 0.5;
	const ax = Math.abs(m.nx);
	const ay = Math.abs(m.ny);
	const az = Math.abs(m.nz);
	let axis: string;
	if (az >= ax && az >= ay) axis = m.nz >= 0 ? 'pz' : 'nz';
	else if (ax >= ay) axis = m.nx >= 0 ? 'px' : 'nx';
	else axis = m.ny >= 0 ? 'py' : 'ny';
	return `${Math.round(m.cx / q)},${Math.round(m.cy / q)},${Math.round(m.cz / q)},${axis}`;
}

export interface TagOverride {
	/** Stable signature (tagSignature). */
	k: string;
	/** Tag name (legend-independent: stored by name so legend reorders are safe). */
	tag: SurfaceTag;
}

/** Manual overrides = the triangles whose tag differs from a fresh auto-classify
 *  — the precious hand-edits, keyed stably. De-duped by signature (last wins). */
export function buildTagOverrides(
	meta: TriMeta[],
	tags: ArrayLike<number>,
	autoTags: ArrayLike<number>
): TagOverride[] {
	const map = new Map<string, SurfaceTag>();
	for (let t = 0; t < meta.length; t++) {
		if (tags[t] !== autoTags[t]) map.set(tagSignature(meta[t]), TAG_LEGEND[tags[t]] ?? 'wall');
	}
	return [...map].map(([k, tag]) => ({ k, tag }));
}

/** Re-apply stable-keyed overrides onto a tag array (returns a new array). Any
 *  triangle whose signature matches an override takes that override's tag. */
export function applyTagOverrides(
	meta: TriMeta[],
	tags: ArrayLike<number>,
	overrides: TagOverride[] | null | undefined
): Uint8Array {
	const out = Uint8Array.from(tags as Uint8Array);
	if (!overrides || overrides.length === 0) return out;
	const byKey = new Map<string, SurfaceTag>();
	for (const o of overrides) byKey.set(o.k, o.tag);
	for (let t = 0; t < meta.length; t++) {
		const ov = byKey.get(tagSignature(meta[t]));
		if (ov) out[t] = TAG_INDEX[ov] ?? out[t];
	}
	return out;
}

// --- Selection -------------------------------------------------------------

class UnionFind {
	private parent: Int32Array;
	constructor(n: number) {
		this.parent = new Int32Array(n);
		for (let i = 0; i < n; i++) this.parent[i] = i;
	}
	find(x: number): number {
		let root = x;
		while (this.parent[root] !== root) root = this.parent[root];
		while (this.parent[x] !== root) {
			const next = this.parent[x];
			this.parent[x] = root;
			x = next;
		}
		return root;
	}
	union(a: number, b: number): void {
		const ra = this.find(a);
		const rb = this.find(b);
		if (ra !== rb) this.parent[rb] = ra;
	}
}

/**
 * Connected-component id per triangle, via SPATIAL vertex welding so a mesh that
 * duplicates coincident vertices per-face (flat shading) still groups into whole
 * architectural pieces — click one ceiling triangle, select the whole ceiling.
 */
export function connectedComponents(
	mesh: Pick<BspMesh, 'positions' | 'indices'>,
	quantum = 1e-3
): Int32Array {
	const pos = mesh.positions;
	const idx = mesh.indices;
	const T = triCount(mesh);
	const uf = new UnionFind(T);
	const firstTriAtVertex = new Map<string, number>();
	const q = quantum > 0 ? quantum : 1e-3;
	const key = (vi: number): string => {
		const x = Math.round(pos[vi] / q);
		const y = Math.round(pos[vi + 1] / q);
		const z = Math.round(pos[vi + 2] / q);
		return `${x},${y},${z}`;
	};
	for (let t = 0; t < T; t++) {
		for (let v = 0; v < 3; v++) {
			const vi = idx[t * 3 + v] * 3;
			const k = key(vi);
			const prev = firstTriAtVertex.get(k);
			if (prev === undefined) firstTriAtVertex.set(k, t);
			else uf.union(prev, t);
		}
	}
	const remap = new Map<number, number>();
	const comp = new Int32Array(T);
	let next = 0;
	for (let t = 0; t < T; t++) {
		const root = uf.find(t);
		let id = remap.get(root);
		if (id === undefined) {
			id = next++;
			remap.set(root, id);
		}
		comp[t] = id;
	}
	return comp;
}

export type PlaneCullMode = 'centroid' | 'whole' | 'any';

/**
 * Triangles ABOVE the cull-height plane (world Z = cullZ). 'centroid' (default)
 * cuts the roof and upper walls that lean mostly above the line; 'whole' is
 * conservative (entirely above); 'any' is the most aggressive (any corner above).
 */
export function selectAbovePlane(
	meta: TriMeta[],
	cullZ: number,
	mode: PlaneCullMode = 'centroid'
): number[] {
	const out: number[] = [];
	for (let t = 0; t < meta.length; t++) {
		const m = meta[t];
		const above =
			mode === 'whole' ? m.zMin >= cullZ : mode === 'any' ? m.zMax > cullZ : m.cz >= cullZ;
		if (above) out.push(t);
	}
	return out;
}

/** All triangles in the same connected component as `tri`. */
export function selectComponent(comp: Int32Array, tri: number): number[] {
	if (tri < 0 || tri >= comp.length) return [];
	const target = comp[tri];
	const out: number[] = [];
	for (let t = 0; t < comp.length; t++) if (comp[t] === target) out.push(t);
	return out;
}

/** All triangles whose averaged material colour is within `tol` of `tri`'s. */
export function selectByMaterial(meta: TriMeta[], tri: number, tol = 0.12): number[] {
	if (tri < 0 || tri >= meta.length) return [];
	const base = meta[tri];
	const t2 = tol * tol;
	const out: number[] = [];
	for (let t = 0; t < meta.length; t++) {
		const m = meta[t];
		const dr = m.r - base.r;
		const dg = m.g - base.g;
		const db = m.b - base.b;
		if (dr * dr + dg * dg + db * db <= t2) out.push(t);
	}
	return out;
}

/** All triangles currently carrying tag `tag`. */
export function selectByTag(tags: ArrayLike<number>, tag: SurfaceTag): number[] {
	const idx = TAG_INDEX[tag];
	const out: number[] = [];
	for (let t = 0; t < tags.length; t++) if (tags[t] === idx) out.push(t);
	return out;
}

/** Keyboard modifiers carried with a pick/box-select gesture (additive vs replace). */
export interface PickMods {
	shift: boolean;
	alt: boolean;
}

/** Screen-rect (inclusive) hit test — small shared helper for box select. */
export interface ScreenRect {
	minX: number;
	minY: number;
	maxX: number;
	maxY: number;
}

export function pointInRect(x: number, y: number, rect: ScreenRect): boolean {
	return x >= rect.minX && x <= rect.maxX && y >= rect.minY && y <= rect.maxY;
}

/** Box select: every triangle whose centroid projects (via the caller's camera
 *  projector) to a screen point inside `rect`. Pure by injecting the projector. */
export function boxSelect(
	meta: TriMeta[],
	project: (cx: number, cy: number, cz: number) => [number, number] | null,
	rect: ScreenRect
): number[] {
	const out: number[] = [];
	for (let t = 0; t < meta.length; t++) {
		const m = meta[t];
		const p = project(m.cx, m.cy, m.cz);
		if (p && pointInRect(p[0], p[1], rect)) out.push(t);
	}
	return out;
}

// --- Tag-driven floor reference + banding -----------------------------------

export interface FloorMarker {
	x: number;
	y: number;
	z: number;
}

/** Centroids of FLOOR + RAMP triangles — the player-accessible surfaces that the
 *  editor shows as a height reference and the elevation banding is computed from
 *  (tags drive the banding). */
export function taggedFloorMarkers(meta: TriMeta[], tags: ArrayLike<number>): FloorMarker[] {
	const floor = TAG_INDEX.floor;
	const ramp = TAG_INDEX.ramp;
	const out: FloorMarker[] = [];
	for (let t = 0; t < meta.length; t++) {
		if (tags[t] === floor || tags[t] === ramp)
			out.push({ x: meta[t].cx, y: meta[t].cy, z: meta[t].cz });
	}
	return out;
}

export function taggedFloorZs(meta: TriMeta[], tags: ArrayLike<number>): number[] {
	return taggedFloorMarkers(meta, tags).map((m) => m.z);
}

/**
 * Default cull-height for a freshly-loaded map: the highest tagged floor/ramp
 * surface plus a headroom margin — i.e. just under the ceiling, so the first plane
 * drag previews the roof as removable. Clamped inside the mesh Z range.
 */
export function defaultCullZ(
	mesh: BspMesh,
	meta: TriMeta[],
	tags: ArrayLike<number>,
	margin = 2.5
): number {
	const { minZ, maxZ } = mesh.bounds;
	let topFloor = -Infinity;
	const floor = TAG_INDEX.floor;
	const ramp = TAG_INDEX.ramp;
	for (let t = 0; t < meta.length; t++)
		if (tags[t] === floor || tags[t] === ramp) topFloor = Math.max(topFloor, meta[t].zMax);
	const cand = Number.isFinite(topFloor) ? topFloor + margin : minZ + (maxZ - minZ) * 0.6;
	const lo = minZ + 0.1;
	const hi = maxZ - 0.05;
	if (!Number.isFinite(cand)) return Math.max(lo, Math.min(hi, (minZ + maxZ) / 2));
	return Math.max(lo, Math.min(hi, cand));
}

// --- Bake / export ----------------------------------------------------------

/** A triangle survives the bake when it isn't manually removed AND its tag's
 *  render rule is on. This is the single predicate the editor + export share. */
export function isBaked(
	t: number,
	removed: ArrayLike<number>,
	tags: ArrayLike<number>,
	tagRender: TagRenderMap
): boolean {
	if (removed[t]) return false;
	return !!tagRender[TAG_LEGEND[tags[t]] ?? 'wall'];
}

export interface SpectatorBand {
	index: number;
	minZ: number;
	maxZ: number;
	midZ: number;
}

/** The baked spectator-mesh file. A SUPERSET of the raw mesh schema
 *  (game-geometry's RawMeshFile), so normalizeMesh / loadBspMesh read it
 *  unchanged, plus the tag layer that survives a re-edit. */
export interface SpectatorMesh {
	schema_version: 2;
	kind: 'spectator-mesh';
	generated_by: 'bsp-editor';
	generated_at?: string;
	game: string;
	scenario: string;
	source_map: string;
	source_mesh?: string;
	cull_z?: number;
	bounds: BspMesh['bounds'];
	positions: number[];
	colors?: number[];
	indices: number[];
	vertex_count: number;
	triangle_count: number;
	/** Tag legend + per-kept-triangle tag index (drives tag-aware view styling). */
	tag_legend: SurfaceTag[];
	tags: number[];
	/** Per-tag render rules used for this bake (the viewer's default visibility). */
	tag_render: TagRenderMap;
	/** Manual tag overrides keyed stably (re-applied on a future re-extract). */
	tag_overrides: TagOverride[];
	/** Optional baked elevation bands (from tagged floors). */
	bands?: SpectatorBand[];
}

/** The re-applicable, geometry-independent slice of the tag data — written as a
 *  hand-editable sidecar (`<key>.tags.json`) so manual tags survive a re-extract. */
export interface TagSidecar {
	schema_version: 2;
	kind: 'spectator-tags';
	legend: SurfaceTag[];
	render: TagRenderMap;
	overrides: TagOverride[];
}

export function buildTagSidecar(
	meta: TriMeta[],
	tags: ArrayLike<number>,
	autoTags: ArrayLike<number>,
	tagRender: TagRenderMap
): TagSidecar {
	return {
		schema_version: 2,
		kind: 'spectator-tags',
		legend: TAG_LEGEND.slice(),
		render: { ...tagRender },
		overrides: buildTagOverrides(meta, tags, autoTags)
	};
}

export interface EditState {
	removed: Uint8Array;
	tags: Uint8Array;
}

export interface ExportOptions {
	meta: TriMeta[];
	autoTags: ArrayLike<number>;
	tagRender: TagRenderMap;
	cullZ?: number;
	sourceMesh?: string;
	bands?: SpectatorBand[];
	generatedAt?: string;
}

/**
 * Bake the kept triangles (per {@link isBaked}) into a fresh SpectatorMesh: only
 * referenced vertices are carried over, re-indexed densely, colours preserved,
 * bounds recomputed, each kept triangle's tag embedded, and the manual-override
 * sidecar data folded in for re-edit survival.
 */
export function exportSpectatorMesh(
	mesh: BspMesh,
	state: EditState,
	opts: ExportOptions
): SpectatorMesh {
	const { removed, tags } = state;
	const { tagRender } = opts;
	const pos = mesh.positions;
	const idx = mesh.indices;
	const hasColor = !!mesh.colors && mesh.colors.length === pos.length;
	const col = hasColor ? (mesh.colors as number[]) : undefined;
	const T = triCount(mesh);

	const remap = new Map<number, number>();
	const positions: number[] = [];
	const colors: number[] | undefined = col ? [] : undefined;
	const indices: number[] = [];
	const outTags: number[] = [];

	let minX = Infinity,
		maxX = -Infinity,
		minY = Infinity,
		maxY = -Infinity,
		minZ = Infinity,
		maxZ = -Infinity;

	const pushVertex = (oldV: number): number => {
		let nv = remap.get(oldV);
		if (nv !== undefined) return nv;
		nv = positions.length / 3;
		const o = oldV * 3;
		const x = pos[o],
			y = pos[o + 1],
			z = pos[o + 2];
		positions.push(x, y, z);
		if (colors && col) colors.push(col[o], col[o + 1], col[o + 2]);
		if (x < minX) minX = x;
		if (x > maxX) maxX = x;
		if (y < minY) minY = y;
		if (y > maxY) maxY = y;
		if (z < minZ) minZ = z;
		if (z > maxZ) maxZ = z;
		remap.set(oldV, nv);
		return nv;
	};

	for (let t = 0; t < T; t++) {
		if (!isBaked(t, removed, tags, tagRender)) continue;
		indices.push(pushVertex(idx[t * 3]), pushVertex(idx[t * 3 + 1]), pushVertex(idx[t * 3 + 2]));
		outTags.push(tags[t]);
	}

	const bounds =
		positions.length > 0
			? { minX, maxX, minY, maxY, minZ, maxZ }
			: { minX: 0, maxX: 0, minY: 0, maxY: 0, minZ: 0, maxZ: 0 };

	return {
		schema_version: 2,
		kind: 'spectator-mesh',
		generated_by: 'bsp-editor',
		generated_at: opts.generatedAt,
		game: mesh.game,
		scenario: mesh.scenario,
		source_map: mesh.sourceMap,
		source_mesh: opts.sourceMesh,
		cull_z: opts.cullZ,
		bounds,
		positions,
		colors,
		indices,
		vertex_count: positions.length / 3,
		triangle_count: indices.length / 3,
		tag_legend: TAG_LEGEND.slice(),
		tags: outTags,
		tag_render: { ...tagRender },
		tag_overrides: buildTagOverrides(opts.meta, tags, opts.autoTags),
		bands: opts.bands
	};
}

export interface EditStats {
	totalTris: number;
	keptTris: number;
	removedTris: number;
	keptVerts: number;
}

/** Kept (baked) / dropped tallies for the editor HUD. */
export function editStats(
	mesh: BspMesh,
	removed: ArrayLike<number>,
	tags: ArrayLike<number>,
	tagRender: TagRenderMap
): EditStats {
	const T = triCount(mesh);
	const used = new Set<number>();
	let kept = 0;
	for (let t = 0; t < T; t++) {
		if (!isBaked(t, removed, tags, tagRender)) continue;
		kept++;
		used.add(mesh.indices[t * 3]);
		used.add(mesh.indices[t * 3 + 1]);
		used.add(mesh.indices[t * 3 + 2]);
	}
	return { totalTris: T, keptTris: kept, removedTris: T - kept, keptVerts: used.size };
}

// --- Undo / redo ------------------------------------------------------------

/**
 * Snapshot-based undo/redo over the per-triangle edit state ({removed, tags}).
 * Each committed edit pushes a deep copy; cheap (a few KB per snapshot even on
 * big maps) and bulletproof vs. command-inversion bugs. Bounded so history can't
 * grow without limit. The CURRENT state is the top of the past stack.
 */
export class EditHistory {
	private past: EditState[] = [];
	private future: EditState[] = [];
	private readonly cap: number;

	constructor(initial: EditState, cap = 100) {
		this.past.push(EditHistory.copy(initial));
		this.cap = Math.max(2, cap);
	}

	private static copy(s: EditState): EditState {
		return { removed: Uint8Array.from(s.removed), tags: Uint8Array.from(s.tags) };
	}

	/** Current state (a deep copy — safe for the caller to mutate before commit). */
	current(): EditState {
		return EditHistory.copy(this.past[this.past.length - 1]);
	}

	/** Commit a new state as the next history step (clears the redo stack). */
	commit(state: EditState): void {
		this.past.push(EditHistory.copy(state));
		this.future = [];
		if (this.past.length > this.cap) this.past.shift();
	}

	canUndo(): boolean {
		return this.past.length > 1;
	}
	canRedo(): boolean {
		return this.future.length > 0;
	}

	undo(): EditState | null {
		if (!this.canUndo()) return null;
		this.future.push(this.past.pop() as EditState);
		return EditHistory.copy(this.past[this.past.length - 1]);
	}

	redo(): EditState | null {
		if (!this.canRedo()) return null;
		const state = this.future.pop() as EditState;
		this.past.push(state);
		return EditHistory.copy(state);
	}
}
