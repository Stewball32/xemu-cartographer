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

/** Auto-classify thresholds on the ABSOLUTE up-component (|nz|) — sign-independent
 *  so flipped winding / double-sided BSP surfaces classify the same. */
const FLOOR_COS = 0.85; // within ~32° of horizontal → floor/ceiling candidate
const RAMP_COS = 0.45; // ~32°–63° → ramp/stairs (walkable slope)
/** A surface this far ABOVE a near-horizontal one (over its footprint) makes the
 *  lower one ENCLOSED → it's a floor you're standing under a ceiling on, not a
 *  roof. Must exceed standing clearance so a real floor is never called a ceiling. */
const ENCLOSE_HEAD = 0.8;

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
	/** XY footprint bbox (the enclosure test + non-Z plane culls read these). */
	minX: number;
	maxX: number;
	minY: number;
	maxY: number;
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
			minX: Math.min(ax, bx, cx),
			maxX: Math.max(ax, bx, cx),
			minY: Math.min(ay, by, cy),
			maxY: Math.max(ay, by, cy),
			r,
			g,
			b: bb
		};
	}
	return out;
}

// --- Auto-classification ----------------------------------------------------

/**
 * First-pass tag per triangle. Orientation comes from the ABSOLUTE up-component
 * (|nz|), so flipped winding / double-sided / single-sided-but-inconsistent BSP
 * surfaces all classify identically — a horizontal surface is NEVER called a
 * ceiling just because its normal happens to point down.
 *
 *   - near-vertical → wall
 *   - sloped (walkable angle) → ramp  (always kept — a slope is walkable)
 *   - near-horizontal → floor if ENCLOSED (another horizontal surface sits
 *     >ENCLOSE_HEAD above it over its footprint — i.e. you stand under a ceiling),
 *     else ceiling (it's the TOPMOST surface there → roof/ceiling).
 *
 * This is the winding-independent floor/ceiling discriminator: a floor inside the
 * play volume has something above it; only the top surface is roof/ceiling. So a
 * mid-level floor with downward-pointing normals stays a floor. inaccessible /
 * clutter are never auto-assigned — they're manual bins.
 */
export function autoClassify(meta: TriMeta[]): Uint8Array {
	const T = meta.length;
	const tags = new Uint8Array(T);
	if (T === 0) return tags;

	// Enclosure grid: bucket every near-horizontal surface's height into the XY
	// cells its footprint covers, so a surface can ask "is anything overhead?".
	let minX = Infinity,
		maxX = -Infinity,
		minY = Infinity,
		maxY = -Infinity;
	for (const m of meta) {
		if (m.minX < minX) minX = m.minX;
		if (m.maxX > maxX) maxX = m.maxX;
		if (m.minY < minY) minY = m.minY;
		if (m.maxY > maxY) maxY = m.maxY;
	}
	const span = Math.max(maxX - minX, maxY - minY, 1e-3);
	const G = Math.min(1.5, Math.max(0.3, span / 64));
	const cols = Math.max(1, Math.ceil((maxX - minX) / G) + 1);
	const rows = Math.max(1, Math.ceil((maxY - minY) / G) + 1);
	const cells: number[][] = Array.from({ length: cols * rows }, () => []);
	const cellRange = (m: TriMeta) => {
		const i0 = Math.max(0, Math.floor((m.minX - minX) / G));
		const i1 = Math.min(cols - 1, Math.floor((m.maxX - minX) / G));
		const j0 = Math.max(0, Math.floor((m.minY - minY) / G));
		const j1 = Math.min(rows - 1, Math.floor((m.maxY - minY) / G));
		return { i0, i1, j0, j1 };
	};
	for (let t = 0; t < T; t++) {
		if (Math.abs(meta[t].nz) < FLOOR_COS) continue; // only near-horizontal decks
		const { i0, i1, j0, j1 } = cellRange(meta[t]);
		for (let i = i0; i <= i1; i++)
			for (let j = j0; j <= j1; j++) cells[j * cols + i].push(meta[t].cz);
	}
	const hasSurfaceAbove = (m: TriMeta): boolean => {
		const { i0, i1, j0, j1 } = cellRange(m);
		for (let i = i0; i <= i1; i++)
			for (let j = j0; j <= j1; j++) {
				const arr = cells[j * cols + i];
				for (const z of arr) if (z - m.cz > ENCLOSE_HEAD) return true;
			}
		return false;
	};

	for (let t = 0; t < T; t++) {
		const up = Math.abs(meta[t].nz);
		let tag: SurfaceTag;
		if (up < RAMP_COS) tag = 'wall';
		else if (up < FLOOR_COS) tag = 'ramp';
		else tag = hasSurfaceAbove(meta[t]) ? 'floor' : 'ceiling';
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
	// Dominant axis WITHOUT sign — so a winding flip on re-extract (which negates
	// the normal) keeps the same key, and the surface's manual tag still re-applies.
	const axis = az >= ax && az >= ay ? 'z' : ax >= ay ? 'x' : 'y';
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

/**
 * Triangle adjacency via spatial vertex welding: `adj[t]` is the list of triangles
 * sharing at least one welded vertex with t. Precomputed once per mesh (like
 * connectedComponents) so the coplanar flood-fill is a cheap graph walk per click.
 */
export function buildAdjacency(
	mesh: Pick<BspMesh, 'positions' | 'indices'>,
	quantum = 1e-3
): number[][] {
	const pos = mesh.positions;
	const idx = mesh.indices;
	const T = triCount(mesh);
	const q = quantum > 0 ? quantum : 1e-3;
	const trisAtVertex = new Map<string, number[]>();
	const key = (vi: number): string =>
		`${Math.round(pos[vi] / q)},${Math.round(pos[vi + 1] / q)},${Math.round(pos[vi + 2] / q)}`;
	for (let t = 0; t < T; t++) {
		for (let v = 0; v < 3; v++) {
			const k = key(idx[t * 3 + v] * 3);
			const arr = trisAtVertex.get(k);
			if (arr) arr.push(t);
			else trisAtVertex.set(k, [t]);
		}
	}
	const adj: Set<number>[] = Array.from({ length: T }, () => new Set<number>());
	for (const arr of trisAtVertex.values()) {
		for (let i = 0; i < arr.length; i++)
			for (let j = i + 1; j < arr.length; j++) {
				adj[arr[i]].add(arr[j]);
				adj[arr[j]].add(arr[i]);
			}
	}
	return adj.map((s) => [...s]);
}

/**
 * Select the whole flat face the clicked triangle belongs to — connected
 * triangles that share its plane (normal within `angleTolDeg`, centroid within
 * `planeTol` of the seed plane) reached only through adjacency. "Selecting an
 * entire side of a cube." Orientation uses |dot| so a coplanar face with mixed
 * winding stays together, while the plane-distance test keeps the opposite face
 * of a thin slab out.
 */
export function selectCoplanar(
	adj: number[][],
	meta: TriMeta[],
	seed: number,
	angleTolDeg = 8,
	planeTol = 0.2
): number[] {
	if (seed < 0 || seed >= meta.length || !adj[seed]) return [];
	const s = meta[seed];
	const cosTol = Math.cos((Math.max(0, angleTolDeg) * Math.PI) / 180);
	const visited = new Uint8Array(meta.length);
	const out: number[] = [];
	const stack = [seed];
	visited[seed] = 1;
	while (stack.length > 0) {
		const t = stack.pop() as number;
		out.push(t);
		for (const nb of adj[t]) {
			if (visited[nb]) continue;
			const m = meta[nb];
			const dot = Math.abs(m.nx * s.nx + m.ny * s.ny + m.nz * s.nz);
			if (dot < cosTol) continue;
			const dist = Math.abs((m.cx - s.cx) * s.nx + (m.cy - s.cy) * s.ny + (m.cz - s.cz) * s.nz);
			if (dist > planeTol) continue;
			visited[nb] = 1;
			stack.push(nb);
		}
	}
	return out.sort((a, b) => a - b);
}

export type PlaneCullMode = 'centroid' | 'whole' | 'any';
export type CullAxis = 'x' | 'y' | 'z';

/**
 * Triangles BEYOND a clip plane perpendicular to `axis` at `offset`, on the side
 * given by `sign` (+1 = the positive/greater side, −1 = the negative/lesser side).
 * Orient on X/Y/Z and flip the sign to clip out-of-bounds geometry from any of the
 * six sides; apply several in sequence to carve a clip box. 'centroid' (default)
 * is the natural cut; 'whole' is conservative (entirely beyond); 'any' is the most
 * aggressive (any corner beyond).
 */
export function selectBeyondPlane(
	meta: TriMeta[],
	axis: CullAxis,
	offset: number,
	sign: 1 | -1 = 1,
	mode: PlaneCullMode = 'centroid'
): number[] {
	const out: number[] = [];
	for (let t = 0; t < meta.length; t++) {
		const m = meta[t];
		const c = axis === 'x' ? m.cx : axis === 'y' ? m.cy : m.cz;
		const lo = axis === 'x' ? m.minX : axis === 'y' ? m.minY : m.zMin;
		const hi = axis === 'x' ? m.maxX : axis === 'y' ? m.maxY : m.zMax;
		let beyond: boolean;
		if (sign >= 0)
			beyond = mode === 'whole' ? lo >= offset : mode === 'any' ? hi > offset : c >= offset;
		else beyond = mode === 'whole' ? hi <= offset : mode === 'any' ? lo < offset : c <= offset;
		if (beyond) out.push(t);
	}
	return out;
}

/** Back-compat shorthand: triangles above the horizontal cull-height plane (Z). */
export function selectAbovePlane(
	meta: TriMeta[],
	cullZ: number,
	mode: PlaneCullMode = 'centroid'
): number[] {
	return selectBeyondPlane(meta, 'z', cullZ, 1, mode);
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
