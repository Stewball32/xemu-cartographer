// Pure editing model for the BSP spectator-mesh editor (/bsp-editor).
//
// The editor loads the SAME structure-BSP mesh the visualizers consume
// ($lib/utils/game-geometry's BspMesh: flat positions/indices/colors in Halo
// world coords) and lets the operator CULL it down to a clean "spectator mesh":
// drop the roof/ceilings + out-of-bounds clutter, keep the walkable interior.
// The output is a culled BspMesh in the exact same JSON schema, so BOTH the 2D
// floorplan (buildFloorplan) and the 3D scene (Scene3D) render it with no view
// changes — the baked asset just replaces the raw one through the loader.
//
// Everything here is PURE / IO-free / DOM-free so the parts that must be correct
// — connectivity, selection, vertex re-indexing on export, undo/redo — unit-test
// without a browser or a GPU. EditorScene.svelte does only the Three.js wiring on
// top of these (picking, the draggable plane, box-drag rectangles).
//
// A triangle is the unit of editing. We index triangles by their ORDINAL
// t = 0..triCount-1; triangle t spans mesh.indices[3t..3t+2]. "Removed" triangles
// are excluded from the export but fully restorable (undo, or the ghost view)
// until the operator bakes the asset — nothing is destructive in the editor.

import type { BspMesh } from '$lib/utils/game-geometry';
import { buildFloorplan } from '$lib/utils/floorplan';

/** Per-triangle precomputed geometry + material, derived once from the mesh. */
export interface TriMeta {
	/** Centroid in Halo world coords. */
	cx: number;
	cy: number;
	cz: number;
	/** Corner Z extent (the plane cull + band tagging read these). */
	zMin: number;
	zMax: number;
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

/** Build per-triangle centroid / Z-extent / material colour for every triangle. */
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
		const az = pos[a + 2],
			bz = pos[b + 2],
			cz = pos[c + 2];
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
			cx: (pos[a] + pos[b] + pos[c]) / 3,
			cy: (pos[a + 1] + pos[b + 1] + pos[c + 1]) / 3,
			cz: (az + bz + cz) / 3,
			zMin: Math.min(az, bz, cz),
			zMax: Math.max(az, bz, cz),
			r,
			g,
			b: bb
		};
	}
	return out;
}

// --- Connectivity ----------------------------------------------------------

class UnionFind {
	private parent: Int32Array;
	constructor(n: number) {
		this.parent = new Int32Array(n);
		for (let i = 0; i < n; i++) this.parent[i] = i;
	}
	find(x: number): number {
		let root = x;
		while (this.parent[root] !== root) root = this.parent[root];
		// Path compression.
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
 * Two triangles join when they share a welded vertex (positions quantised to
 * `quantum` world units). Returns a normalised id array (0..k-1, in first-seen
 * order) so the result is stable for tests + colouring.
 */
export function connectedComponents(
	mesh: Pick<BspMesh, 'positions' | 'indices'>,
	quantum = 1e-3
): Int32Array {
	const pos = mesh.positions;
	const idx = mesh.indices;
	const T = triCount(mesh);
	const uf = new UnionFind(T);

	// Welded-vertex key → first triangle that touched it; union subsequent ones.
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

	// Normalise root ids → dense 0..k-1 in first-appearance order.
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

// --- Selection -------------------------------------------------------------

export type PlaneCullMode = 'centroid' | 'whole' | 'any';

/**
 * Triangles ABOVE the cull-height plane (world Z = cullZ). The three modes trade
 * how aggressively a triangle that straddles the plane is treated:
 *   - 'centroid' (default): centroid above — cuts the roof AND upper walls that
 *     lean mostly above the line; the cleanest one-drag roof removal.
 *   - 'whole': only triangles ENTIRELY above (zMin ≥ cullZ) — conservative, never
 *     touches anything dipping below the line.
 *   - 'any': any corner above — most aggressive.
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

/**
 * All triangles whose averaged material colour is within `tol` (Euclidean in the
 * 0..1 RGB cube) of `tri`'s — "select the whole ceiling by its texture". Material
 * is global (not limited to a connected piece) so a repeated wall/ceiling texture
 * is caught everywhere it appears.
 */
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

/**
 * Box select: every triangle whose centroid projects (via the caller's camera
 * projector) to a screen point inside `rect`. `project` returns null for points
 * behind the camera / off-NDC so they're skipped. Kept pure by injecting the
 * projector — the component passes a Three camera projection, tests pass a fake.
 */
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

// --- Default cull height ----------------------------------------------------

/**
 * A sensible default cull-height for a freshly-loaded map: the highest WALKABLE
 * floor (the surfaces players actually stand on, from the shared floorplan
 * extractor) plus a headroom margin — i.e. just under the ceiling, so the first
 * drag previews the roof as removable. This is the offline analogue of the live
 * "highest spawn Z + headroom" heuristic (no live feed in the editor, so floors
 * stand in for spawns). Clamped within the mesh's Z range.
 */
export function defaultCullZ(mesh: BspMesh, margin = 2.5): number {
	const { minZ, maxZ } = mesh.bounds;
	let cand: number;
	try {
		const fp = buildFloorplan(mesh);
		const zs = fp.floorZs.filter((z) => Number.isFinite(z));
		cand = zs.length > 0 ? Math.max(...zs) + margin : minZ + (maxZ - minZ) * 0.6;
	} catch {
		cand = minZ + (maxZ - minZ) * 0.6;
	}
	// Keep it inside the slider range and strictly below the very top so the roof
	// at/near maxZ always previews as removable.
	const lo = minZ + 0.1;
	const hi = maxZ - 0.05;
	if (!Number.isFinite(cand)) return Math.max(lo, Math.min(hi, (minZ + maxZ) / 2));
	return Math.max(lo, Math.min(hi, cand));
}

// --- Floor reference markers (player-accessible height cue) ------------------

export interface FloorMarker {
	x: number;
	y: number;
	z: number;
}

/** Walkable-floor centroids — the editor shows these as the "where players can
 *  stand" reference while the operator drags the cull plane. Reuses the shared
 *  floorplan extractor so they match the 2D view's walkable surfaces exactly. */
export function floorMarkers(mesh: BspMesh): FloorMarker[] {
	try {
		const fp = buildFloorplan(mesh);
		return fp.floors.map((f) => ({
			x: (f.pts[0].x + f.pts[1].x + f.pts[2].x) / 3,
			y: (f.pts[0].y + f.pts[1].y + f.pts[2].y) / 3,
			z: f.z
		}));
	} catch {
		return [];
	}
}

// --- Export -----------------------------------------------------------------

export interface SpectatorBand {
	index: number;
	minZ: number;
	maxZ: number;
	midZ: number;
}

/** The baked spectator-mesh file. A strict SUPERSET of the raw mesh schema
 *  (game-geometry's RawMeshFile), so normalizeMesh / loadBspMesh read it with no
 *  changes, and the editor can re-import it to keep iterating. */
export interface SpectatorMesh {
	schema_version: 1;
	kind: 'spectator-mesh';
	generated_by: 'bsp-editor';
	generated_at?: string;
	game: string;
	scenario: string;
	source_map: string;
	/** The raw mesh file this was baked from (provenance). */
	source_mesh?: string;
	/** Cull-height the operator settled on (provenance / re-open hint). */
	cull_z?: number;
	bounds: BspMesh['bounds'];
	positions: number[];
	colors?: number[];
	indices: number[];
	vertex_count: number;
	triangle_count: number;
	/** Optional baked elevation bands for the shading (else the viewer recomputes). */
	bands?: SpectatorBand[];
}

export interface ExportOptions {
	cullZ?: number;
	sourceMesh?: string;
	bands?: SpectatorBand[];
	generatedAt?: string;
}

/**
 * Bake the kept (non-removed) triangles into a fresh SpectatorMesh: only the
 * referenced vertices are carried over, re-indexed densely (no vertex explosion,
 * shared verts stay shared), colours preserved when present, bounds recomputed.
 * `removed[t]` truthy ⇒ triangle t is dropped.
 */
export function exportSpectatorMesh(
	mesh: BspMesh,
	removed: ArrayLike<number>,
	opts: ExportOptions = {}
): SpectatorMesh {
	const pos = mesh.positions;
	const idx = mesh.indices;
	const hasColor = !!mesh.colors && mesh.colors.length === pos.length;
	const col = hasColor ? (mesh.colors as number[]) : undefined;
	const T = triCount(mesh);

	const remap = new Map<number, number>(); // old vertex index → new vertex index
	const positions: number[] = [];
	const colors: number[] | undefined = col ? [] : undefined;
	const indices: number[] = [];

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
		if (removed[t]) continue;
		const a = idx[t * 3];
		const b = idx[t * 3 + 1];
		const c = idx[t * 3 + 2];
		indices.push(pushVertex(a), pushVertex(b), pushVertex(c));
	}

	const bounds =
		positions.length > 0
			? { minX, maxX, minY, maxY, minZ, maxZ }
			: { minX: 0, maxX: 0, minY: 0, maxY: 0, minZ: 0, maxZ: 0 };

	return {
		schema_version: 1,
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
		bands: opts.bands
	};
}

export interface EditStats {
	totalTris: number;
	keptTris: number;
	removedTris: number;
	keptVerts: number;
}

/** Quick kept/removed tallies for the editor HUD. */
export function editStats(mesh: BspMesh, removed: ArrayLike<number>): EditStats {
	const T = triCount(mesh);
	const used = new Set<number>();
	let removedTris = 0;
	for (let t = 0; t < T; t++) {
		if (removed[t]) {
			removedTris++;
			continue;
		}
		used.add(mesh.indices[t * 3]);
		used.add(mesh.indices[t * 3 + 1]);
		used.add(mesh.indices[t * 3 + 2]);
	}
	return { totalTris: T, keptTris: T - removedTris, removedTris, keptVerts: used.size };
}

// --- Undo / redo ------------------------------------------------------------

/**
 * Snapshot-based undo/redo over the per-triangle removed-state. Each committed
 * edit pushes a copy of the Uint8Array; cheap (a few KB even for big maps) and
 * bulletproof vs. command-inversion bugs. Bounded so history can't grow without
 * limit. The CURRENT state is the top of the past stack.
 */
export class RemovedHistory {
	private past: Uint8Array[] = [];
	private future: Uint8Array[] = [];
	private readonly cap: number;

	constructor(initial: Uint8Array, cap = 100) {
		this.past.push(Uint8Array.from(initial));
		this.cap = Math.max(2, cap);
	}

	/** Current removed-state (a copy — safe to mutate by the caller before commit). */
	current(): Uint8Array {
		return Uint8Array.from(this.past[this.past.length - 1]);
	}

	/** Commit a new state as the next history step (clears the redo stack). */
	commit(state: Uint8Array): void {
		this.past.push(Uint8Array.from(state));
		this.future = [];
		if (this.past.length > this.cap) this.past.shift();
	}

	canUndo(): boolean {
		return this.past.length > 1;
	}
	canRedo(): boolean {
		return this.future.length > 0;
	}

	/** Step back one edit; returns the restored state, or null if nothing to undo. */
	undo(): Uint8Array | null {
		if (!this.canUndo()) return null;
		this.future.push(this.past.pop() as Uint8Array);
		return Uint8Array.from(this.past[this.past.length - 1]);
	}

	/** Step forward one edit; returns the restored state, or null if none. */
	redo(): Uint8Array | null {
		if (!this.canRedo()) return null;
		const state = this.future.pop() as Uint8Array;
		this.past.push(state);
		return Uint8Array.from(state);
	}
}
