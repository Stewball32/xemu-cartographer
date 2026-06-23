import { describe, it, expect } from 'vitest';
import {
	slugifyTagSegment,
	iconKeyForItem,
	iconKeyForVehicle,
	loadIconSet,
	emptyIconSet,
	OBJECTIVE_FLAG_KEY
} from './game-icons';

describe('slugifyTagSegment', () => {
	it('lowercases and collapses non-alphanumerics to underscores', () => {
		expect(slugifyTagSegment('Assault Rifle')).toBe('assault_rifle');
		expect(slugifyTagSegment('rocket_launcher')).toBe('rocket_launcher');
		expect(slugifyTagSegment('  Plasma  Pistol  ')).toBe('plasma_pistol');
		expect(slugifyTagSegment('over-shield!')).toBe('over_shield');
	});
});

describe('iconKeyForItem', () => {
	it('keys weapons by their tag folder (second path segment)', () => {
		expect(iconKeyForItem('weapons\\pistol\\pistol', 'weapon')).toBe('weapon_pistol');
		expect(iconKeyForItem('weapons\\assault rifle\\assault rifle', 'weapon')).toBe(
			'weapon_assault_rifle'
		);
		// extractor uses the FOLDER, so a differing leaf name still matches
		expect(iconKeyForItem('weapons\\rocket launcher\\rocket_launcher', 'weapon')).toBe(
			'weapon_rocket_launcher'
		);
	});

	it('detects grenades by substring regardless of classified kind', () => {
		// CE files grenades under weapons\\ so classifyTag returns 'weapon'
		expect(iconKeyForItem('weapons\\frag grenade\\frag grenade', 'weapon')).toBe('grenade');
		expect(iconKeyForItem('weapons\\plasma grenade\\plasma grenade', 'weapon')).toBe('grenade');
	});

	it('keys powerups / equipment optimistically (gated later by the manifest)', () => {
		expect(iconKeyForItem('powerups\\over shield\\over shield', 'powerup')).toBe(
			'powerup_over_shield'
		);
		expect(iconKeyForItem('powerups\\active camouflage', 'powerup')).toBe(
			'powerup_active_camouflage'
		);
	});

	it('returns null for kinds with no per-type icon', () => {
		expect(iconKeyForItem('characters\\cyborg\\cyborg', 'biped')).toBeNull();
		expect(iconKeyForItem('', 'weapon')).toBeNull();
		expect(iconKeyForItem(null, 'weapon')).toBeNull();
	});

	it('forward slashes resolve the same as backslashes', () => {
		expect(iconKeyForItem('weapons/shotgun/shotgun', 'weapon')).toBe('weapon_shotgun');
	});
});

describe('iconKeyForVehicle', () => {
	it('keys vehicles by folder (manifest decides if the sprite exists)', () => {
		expect(iconKeyForVehicle('vehicles\\warthog\\warthog')).toBe('vehicle_warthog');
		expect(iconKeyForVehicle('vehicles\\ghost\\ghost')).toBe('vehicle_ghost');
		expect(iconKeyForVehicle('')).toBeNull();
	});
});

describe('emptyIconSet', () => {
	it('never resolves a url', () => {
		const s = emptyIconSet('haloce');
		expect(s.loaded).toBe(false);
		expect(s.url('weapon_pistol')).toBeNull();
		expect(s.url(OBJECTIVE_FLAG_KEY)).toBeNull();
	});
});

describe('loadIconSet', () => {
	const manifest = {
		game: 'haloce',
		icons: {
			weapon_pistol: { file: 'weapon_pistol.png' },
			objective_flag: { file: 'objective_flag.png' }
		}
	};

	it('resolves served URLs for keys present in the manifest', async () => {
		const fetchFn = (async () =>
			new Response(JSON.stringify(manifest), { status: 200 })) as unknown as typeof fetch;
		const set = await loadIconSet('haloce', fetchFn);
		expect(set.loaded).toBe(true);
		expect(set.url('weapon_pistol')).toBe('/game-icons/haloce/weapon_pistol.png');
		expect(set.url(OBJECTIVE_FLAG_KEY)).toBe('/game-icons/haloce/objective_flag.png');
		// a key the manifest doesn't ship → null (fallback marker)
		expect(set.url('vehicle_warthog')).toBeNull();
		expect(set.url(null)).toBeNull();
	});

	it('falls back to an empty set when the cache is missing (404)', async () => {
		const fetchFn = (async () =>
			new Response('not found', { status: 404 })) as unknown as typeof fetch;
		const set = await loadIconSet('haloce', fetchFn);
		expect(set.loaded).toBe(false);
		expect(set.url('weapon_pistol')).toBeNull();
	});

	it('falls back to an empty set when fetch throws (offline)', async () => {
		const fetchFn = (async () => {
			throw new Error('network down');
		}) as unknown as typeof fetch;
		const set = await loadIconSet('haloce', fetchFn);
		expect(set.loaded).toBe(false);
		expect(set.url('weapon_pistol')).toBeNull();
	});
});
