// Scenario → real-level-mesh lookup for the 3D visualizer.
//
// The actual geometry (structure-BSP rendered surfaces) is extracted from the
// user's own Halo `.map` files by scripts/game-geometry/extract_bsp.py into a
// git-ignored, visualizer-served cache (sveltekit/static/game-geometry/<game>/)
// with a manifest.json — exactly the stance the HUD icons (game-icons.ts) and
// the map-preview PNGs take. This module is the consumer: a PURE scenario→key
// derivation (unit-tested, no IO) plus a thin, failure-tolerant loader.
//
// Design: if the cache was never regenerated (no `.map` files on this machine),
// loadBspMesh resolves to null and the scene degrades to the auto-fit world-
// bounds box — the live markers still render, just without the level shell. A
// new map (or H2) drops in as another manifest entry with no frontend change.

import { base } from '$app/paths';
import type { WorldBounds } from '$lib/utils/visualizer-view';

/** Slugify a scenario path segment to a mesh key — MUST match the extractor's
 * Python slugify (lowercase, non-alphanumeric runs → '_', trimmed). */
export function slugifyScenarioSegment(s: string): string {
	return s
		.trim()
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, '_')
		.replace(/^_+|_+$/g, '');
}

/**
 * Derive the mesh key for a scenario/map string. The reader reports the full
 * tag path ("levels\\test\\bloodgulch\\bloodgulch"); the level is identified by
 * its last segment ("bloodgulch"). Returns '' for empty input so the caller can
 * skip the fetch.
 */
export function meshKeyForScenario(scenario: string | null | undefined): string {
	const parts = (scenario ?? '').split(/[\\/]/).filter(Boolean);
	if (parts.length === 0) return '';
	return slugifyScenarioSegment(parts[parts.length - 1]);
}

/** A decoded structure-BSP mesh in Halo world coordinates (X/Y ground, Z up) —
 * the SAME frame as the live marker positions, so Scene3D maps both through
 * haloToThree and they line up. Flat arrays for compact transfer + a direct
 * handoff to a Three.js BufferGeometry. */
export interface BspMesh {
	game: string;
	scenario: string;
	sourceMap: string;
	/** Bounds of the mesh in Halo world units (mirrors WorldBounds, minus the
	 *  source/valid bookkeeping). */
	bounds: Pick<WorldBounds, 'minX' | 'maxX' | 'minY' | 'maxY' | 'minZ' | 'maxZ'>;
	/** xyz triples, length = 3 × vertex count. */
	positions: number[];
	/** rgb triples (0..1), length = 3 × vertex count — each vertex's material
	 *  color sampled from the real map texture. Absent on legacy caches. */
	colors?: number[];
	/** Triangle vertex indices into `positions`, length = 3 × triangle count. */
	indices: number[];
	/** Distinct material (shader) names, e.g. "levels\\test\\chillout\\shaders\\
	 *  chillout plate floor". Index space for `triangleMaterial`. Absent on legacy
	 *  caches (schema < 3). */
	materials?: string[];
	/** Coarse spectator semantic per material (parallel to `materials`): one of
	 *  floor/ramp/wall/ceiling/glass/grate/sky/ladder/other — the editor maps these
	 *  to surface-tag priors. */
	materialSemantic?: string[];
	/** Per-triangle index into `materials`, length = triangle count. Lets the
	 *  auto-tagger read each surface's material name as a classification prior. */
	triangleMaterial?: number[];
}

/** The 2D top-down projection of a level's BSP, for the minimap background. The
 *  PNG covers exactly `bounds` (world rect, north up); the consumer places it by
 *  projecting those bounds through the SAME transform the live dots use. */
export interface TopProjection {
	url: string;
	bounds: BspMesh['bounds'];
}

export interface GeometryManifestEntry {
	file: string;
	/** Baked spectator-mesh override (see /bsp-editor). When present, loadBspMesh
	 *  serves THIS culled mesh instead of `file` — so both visualizer views pick
	 *  up the cleaned geometry with no page changes. Absent on raw caches. */
	spectator_file?: string | null;
	top_image?: string | null;
	bounds?: BspMesh['bounds'];
	scenario?: string;
	source_map?: string;
	vertex_count?: number;
	triangle_count?: number;
}

export interface GeometryManifest {
	game: string;
	meshes: Record<string, GeometryManifestEntry>;
}

interface RawMeshFile {
	game?: string;
	scenario?: string;
	source_map?: string;
	bounds?: BspMesh['bounds'];
	positions?: number[];
	colors?: number[];
	indices?: number[];
	materials?: string[];
	material_semantic?: string[];
	triangle_material?: number[];
}

function cacheRoot(game: string): string {
	return `${base}/game-geometry/${game}`;
}

/**
 * Load the structure-BSP mesh for a scenario, or null when it isn't cached.
 * Resilient by design: a never-regenerated cache (404), a scenario the manifest
 * doesn't list, malformed JSON, or a mesh with no triangles all resolve to null
 * so the scene falls back to the world-bounds box rather than throwing.
 *
 * Baked spectator mesh: when the manifest entry carries a `spectator_file` (the
 * culled asset the /bsp-editor produced), it is served INSTEAD of the raw `file`
 * — so both visualizer views render the cleaned geometry with no page changes,
 * and fall back to the raw mesh wherever no baked asset exists. Pass
 * `{ raw: true }` to force the original mesh (the editor edits the raw one).
 */
export async function loadBspMesh(
	game: string,
	scenario: string | null | undefined,
	fetchFn: typeof fetch = fetch,
	opts: { raw?: boolean } = {}
): Promise<BspMesh | null> {
	const key = meshKeyForScenario(scenario);
	if (!key) return null;
	try {
		const manRes = await fetchFn(`${cacheRoot(game)}/manifest.json`);
		if (!manRes.ok) return null;
		const man = (await manRes.json()) as GeometryManifest;
		const entry = man?.meshes?.[key];
		if (!entry?.file) return null;

		const file = !opts.raw && entry.spectator_file ? entry.spectator_file : entry.file;
		const meshRes = await fetchFn(`${cacheRoot(game)}/${file}`);
		if (!meshRes.ok) return null;
		const raw = (await meshRes.json()) as RawMeshFile;
		return normalizeMesh(game, key, raw);
	} catch {
		return null;
	}
}

/**
 * Load the whole geometry manifest for a game (the editor's map picker reads it
 * to list extracted maps + whether each already has a baked spectator mesh).
 * Null when the cache was never generated.
 */
export async function loadGeometryManifest(
	game: string,
	fetchFn: typeof fetch = fetch
): Promise<GeometryManifest | null> {
	try {
		const res = await fetchFn(`${cacheRoot(game)}/manifest.json`);
		if (!res.ok) return null;
		const man = (await res.json()) as GeometryManifest;
		return man?.meshes ? man : null;
	} catch {
		return null;
	}
}

/** Validate + normalize a raw mesh file into a BspMesh, or null if it carries no
 *  usable geometry. Exported for unit-testing the degrade path without IO. */
export function normalizeMesh(
	game: string,
	key: string,
	raw: RawMeshFile | null | undefined
): BspMesh | null {
	if (!raw || !Array.isArray(raw.positions) || !Array.isArray(raw.indices)) return null;
	if (raw.positions.length < 9 || raw.indices.length < 3) return null; // < 1 triangle
	if (raw.positions.length % 3 !== 0 || raw.indices.length % 3 !== 0) return null;
	const bounds = raw.bounds ?? boundsFromPositions(raw.positions);
	// Colors are optional + only used when they line up 1:1 with positions.
	const colors =
		Array.isArray(raw.colors) && raw.colors.length === raw.positions.length
			? raw.colors
			: undefined;
	// Material-name priors (schema 3) — only carried when the per-triangle index
	// is well-formed (one entry per triangle, every index in range), so a stale or
	// half-written cache degrades to no-priors rather than corrupting the tagger.
	const triCountRaw = raw.indices.length / 3;
	const materials = Array.isArray(raw.materials) ? raw.materials : undefined;
	const materialSemantic =
		Array.isArray(raw.material_semantic) &&
		materials &&
		raw.material_semantic.length === materials.length
			? raw.material_semantic
			: undefined;
	const triangleMaterial =
		Array.isArray(raw.triangle_material) &&
		materials &&
		raw.triangle_material.length === triCountRaw &&
		raw.triangle_material.every((i) => Number.isInteger(i) && i >= 0 && i < materials.length)
			? raw.triangle_material
			: undefined;
	return {
		game,
		scenario: raw.scenario ?? key,
		sourceMap: raw.source_map ?? '',
		bounds,
		positions: raw.positions,
		colors,
		indices: raw.indices,
		materials: triangleMaterial ? materials : undefined,
		materialSemantic: triangleMaterial ? materialSemantic : undefined,
		triangleMaterial
	};
}

/**
 * Load the 2D top-down projection (PNG + world bounds) for a scenario, or null
 * when it isn't cached. Lightweight sibling of loadBspMesh for the 2D minimap
 * background: reads only the manifest (no mesh download). Degrades to null on
 * any failure so the minimap keeps its blank grid.
 */
export async function loadTopProjection(
	game: string,
	scenario: string | null | undefined,
	fetchFn: typeof fetch = fetch
): Promise<TopProjection | null> {
	const key = meshKeyForScenario(scenario);
	if (!key) return null;
	try {
		const manRes = await fetchFn(`${cacheRoot(game)}/manifest.json`);
		if (!manRes.ok) return null;
		const man = (await manRes.json()) as GeometryManifest;
		const entry = man?.meshes?.[key];
		if (!entry?.top_image || !entry.bounds) return null;
		return { url: `${cacheRoot(game)}/${entry.top_image}`, bounds: entry.bounds };
	} catch {
		return null;
	}
}

function boundsFromPositions(positions: number[]): BspMesh['bounds'] {
	let minX = Infinity,
		maxX = -Infinity,
		minY = Infinity,
		maxY = -Infinity,
		minZ = Infinity,
		maxZ = -Infinity;
	for (let i = 0; i + 2 < positions.length; i += 3) {
		const x = positions[i],
			y = positions[i + 1],
			z = positions[i + 2];
		if (x < minX) minX = x;
		if (x > maxX) maxX = x;
		if (y < minY) minY = y;
		if (y > maxY) maxY = y;
		if (z < minZ) minZ = z;
		if (z > maxZ) maxZ = z;
	}
	return { minX, maxX, minY, maxY, minZ, maxZ };
}
