import { describe, it, expect } from 'vitest';
import {
	slugifyScenarioSegment,
	meshKeyForScenario,
	normalizeMesh,
	loadBspMesh,
	loadTopProjection
} from './game-geometry';

describe('slugifyScenarioSegment', () => {
	it('matches the extractor slugify (lowercase, non-alnum → underscore, trimmed)', () => {
		expect(slugifyScenarioSegment('Blood Gulch')).toBe('blood_gulch');
		expect(slugifyScenarioSegment('bloodgulch')).toBe('bloodgulch');
		expect(slugifyScenarioSegment('  Death Island!  ')).toBe('death_island');
	});
});

describe('meshKeyForScenario', () => {
	it('keys by the last path segment of the scenario tag path', () => {
		expect(meshKeyForScenario('levels\\test\\bloodgulch\\bloodgulch')).toBe('bloodgulch');
		expect(meshKeyForScenario('levels/test/sidewinder/sidewinder')).toBe('sidewinder');
		expect(meshKeyForScenario('bloodgulch')).toBe('bloodgulch');
	});

	it('returns empty string for empty/nullish input (caller skips the fetch)', () => {
		expect(meshKeyForScenario('')).toBe('');
		expect(meshKeyForScenario(null)).toBe('');
		expect(meshKeyForScenario(undefined)).toBe('');
	});
});

describe('normalizeMesh', () => {
	const tri = {
		positions: [0, 0, 0, 10, 0, 0, 0, 10, 0],
		indices: [0, 1, 2]
	};

	it('accepts a well-formed mesh and derives bounds when absent', () => {
		const m = normalizeMesh('haloce', 'bloodgulch', tri);
		expect(m).not.toBeNull();
		expect(m!.indices).toEqual([0, 1, 2]);
		expect(m!.bounds).toEqual({ minX: 0, maxX: 10, minY: 0, maxY: 10, minZ: 0, maxZ: 0 });
		// falls back to the key when scenario/source aren't carried
		expect(m!.scenario).toBe('bloodgulch');
	});

	it('keeps per-vertex colors when they match positions length, else drops them', () => {
		const base = { positions: [0, 0, 0, 10, 0, 0, 0, 10, 0], indices: [0, 1, 2] };
		const ok = normalizeMesh('haloce', 'bg', { ...base, colors: [1, 0, 0, 0, 1, 0, 0, 0, 1] });
		expect(ok!.colors).toEqual([1, 0, 0, 0, 1, 0, 0, 0, 1]);
		const bad = normalizeMesh('haloce', 'bg', { ...base, colors: [1, 0, 0] });
		expect(bad!.colors).toBeUndefined();
	});

	it('keeps explicit bounds + scenario/source from the file', () => {
		const m = normalizeMesh('haloce', 'bloodgulch', {
			...tri,
			scenario: 'levels\\test\\bloodgulch\\bloodgulch',
			source_map: 'bloodgulch.map',
			bounds: { minX: -1, maxX: 1, minY: -2, maxY: 2, minZ: -3, maxZ: 3 }
		});
		expect(m!.scenario).toBe('levels\\test\\bloodgulch\\bloodgulch');
		expect(m!.sourceMap).toBe('bloodgulch.map');
		expect(m!.bounds.minZ).toBe(-3);
	});

	it('rejects degenerate / malformed geometry (degrades to box)', () => {
		expect(normalizeMesh('haloce', 'x', null)).toBeNull();
		expect(normalizeMesh('haloce', 'x', { positions: [], indices: [] })).toBeNull();
		expect(normalizeMesh('haloce', 'x', { positions: [0, 0, 0], indices: [0, 1, 2] })).toBeNull(); // < 1 tri of verts
		// non-multiple-of-3 lengths
		expect(
			normalizeMesh('haloce', 'x', { positions: [0, 0, 0, 1, 1], indices: [0, 1, 2] })
		).toBeNull();
	});
});

describe('loadBspMesh', () => {
	const manifest = { game: 'haloce', meshes: { bloodgulch: { file: 'bloodgulch.json' } } };
	const meshFile = {
		game: 'haloce',
		scenario: 'levels\\test\\bloodgulch\\bloodgulch',
		source_map: 'bloodgulch.map',
		bounds: { minX: -70, maxX: 70, minY: -70, maxY: 70, minZ: -5, maxZ: 30 },
		positions: [0, 0, 0, 10, 0, 0, 0, 10, 0],
		indices: [0, 1, 2]
	};

	function routedFetch(): typeof fetch {
		return (async (url: string) => {
			if (url.endsWith('manifest.json'))
				return new Response(JSON.stringify(manifest), { status: 200 });
			if (url.endsWith('bloodgulch.json'))
				return new Response(JSON.stringify(meshFile), { status: 200 });
			return new Response('not found', { status: 404 });
		}) as unknown as typeof fetch;
	}

	it('loads the mesh for a scenario present in the manifest', async () => {
		const m = await loadBspMesh('haloce', 'levels\\test\\bloodgulch\\bloodgulch', routedFetch());
		expect(m).not.toBeNull();
		expect(m!.sourceMap).toBe('bloodgulch.map');
		expect(m!.positions.length).toBe(9);
		expect(m!.bounds.maxZ).toBe(30);
	});

	it('returns null for a scenario the manifest does not list', async () => {
		const m = await loadBspMesh('haloce', 'levels\\test\\sidewinder\\sidewinder', routedFetch());
		expect(m).toBeNull();
	});

	it('returns null when the cache is missing (manifest 404)', async () => {
		const fetchFn = (async () => new Response('nope', { status: 404 })) as unknown as typeof fetch;
		expect(await loadBspMesh('haloce', 'bloodgulch', fetchFn)).toBeNull();
	});

	it('returns null when fetch throws (offline)', async () => {
		const fetchFn = (async () => {
			throw new Error('network down');
		}) as unknown as typeof fetch;
		expect(await loadBspMesh('haloce', 'bloodgulch', fetchFn)).toBeNull();
	});

	it('returns null for an empty scenario without fetching', async () => {
		let called = false;
		const fetchFn = (async () => {
			called = true;
			return new Response('{}', { status: 200 });
		}) as unknown as typeof fetch;
		expect(await loadBspMesh('haloce', '', fetchFn)).toBeNull();
		expect(called).toBe(false);
	});
});

describe('loadTopProjection', () => {
	const manifest = {
		game: 'haloce',
		meshes: {
			bloodgulch: {
				file: 'bloodgulch.json',
				top_image: 'bloodgulch_top.png',
				bounds: { minX: -70, maxX: 70, minY: -70, maxY: 70, minZ: -5, maxZ: 30 }
			}
		}
	};
	function fetchManifest(): typeof fetch {
		return (async (url: string) =>
			url.endsWith('manifest.json')
				? new Response(JSON.stringify(manifest), { status: 200 })
				: new Response('nf', { status: 404 })) as unknown as typeof fetch;
	}

	it('returns the PNG url + world bounds for a cached scenario', async () => {
		const t = await loadTopProjection(
			'haloce',
			'levels\\test\\bloodgulch\\bloodgulch',
			fetchManifest()
		);
		expect(t).not.toBeNull();
		expect(t!.url).toBe('/game-geometry/haloce/bloodgulch_top.png');
		expect(t!.bounds.maxZ).toBe(30);
	});

	it('returns null when the manifest entry has no top_image', async () => {
		const m = { game: 'haloce', meshes: { bloodgulch: { file: 'bloodgulch.json' } } };
		const f = (async () =>
			new Response(JSON.stringify(m), { status: 200 })) as unknown as typeof fetch;
		expect(await loadTopProjection('haloce', 'bloodgulch', f)).toBeNull();
	});

	it('returns null on missing cache (404) or empty scenario', async () => {
		const f404 = (async () => new Response('nf', { status: 404 })) as unknown as typeof fetch;
		expect(await loadTopProjection('haloce', 'bloodgulch', f404)).toBeNull();
		expect(await loadTopProjection('haloce', '', fetchManifest())).toBeNull();
	});
});
