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
	/** Triangle vertex indices into `positions`, length = 3 × triangle count. */
	indices: number[];
}

interface GeometryManifestEntry {
	file: string;
}

interface GeometryManifest {
	game: string;
	meshes: Record<string, GeometryManifestEntry>;
}

interface RawMeshFile {
	game?: string;
	scenario?: string;
	source_map?: string;
	bounds?: BspMesh['bounds'];
	positions?: number[];
	indices?: number[];
}

function cacheRoot(game: string): string {
	return `${base}/game-geometry/${game}`;
}

/**
 * Load the structure-BSP mesh for a scenario, or null when it isn't cached.
 * Resilient by design: a never-regenerated cache (404), a scenario the manifest
 * doesn't list, malformed JSON, or a mesh with no triangles all resolve to null
 * so the scene falls back to the world-bounds box rather than throwing.
 */
export async function loadBspMesh(
	game: string,
	scenario: string | null | undefined,
	fetchFn: typeof fetch = fetch
): Promise<BspMesh | null> {
	const key = meshKeyForScenario(scenario);
	if (!key) return null;
	try {
		const manRes = await fetchFn(`${cacheRoot(game)}/manifest.json`);
		if (!manRes.ok) return null;
		const man = (await manRes.json()) as GeometryManifest;
		const entry = man?.meshes?.[key];
		if (!entry?.file) return null;

		const meshRes = await fetchFn(`${cacheRoot(game)}/${entry.file}`);
		if (!meshRes.ok) return null;
		const raw = (await meshRes.json()) as RawMeshFile;
		return normalizeMesh(game, key, raw);
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
	return {
		game,
		scenario: raw.scenario ?? key,
		sourceMap: raw.source_map ?? '',
		bounds,
		positions: raw.positions,
		indices: raw.indices
	};
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
