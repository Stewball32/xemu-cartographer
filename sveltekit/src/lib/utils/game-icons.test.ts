import { describe, it, expect } from 'vitest';
import {
	slugifyTagSegment,
	iconKeyForItem,
	iconKeyForVehicle,
	resolveItemKey,
	resolveVehicleKey,
	resolveFlagKey,
	loadIconSet,
	emptyIconSet,
	OBJECTIVE_FLAG_KEY,
	type TagMap
} from './game-icons';

const EMPTY: TagMap = {
	weapon: {},
	powerup: {},
	equipment: {},
	grenade: {},
	objective: {},
	vehicle: {}
};

// Mirrors what the CE manifest ships (weapons authoritative, powerups → markers).
const CE: TagMap = {
	weapon: { pistol: 'weapon_pistol', rocket_launcher: 'weapon_rocket_launcher' },
	powerup: {
		over_shield: 'marker_diamond',
		active_camouflage: 'marker_ring',
		health_pack: 'marker_chevron'
	},
	equipment: {},
	grenade: { '*': 'grenade' },
	objective: { flag: 'objective_flag', oddball: 'objective_oddball', koth: 'objective_koth' },
	vehicle: {}
};

describe('slugifyTagSegment', () => {
	it('lowercases and collapses non-alphanumerics to underscores', () => {
		expect(slugifyTagSegment('Over Shield')).toBe('over_shield');
		expect(slugifyTagSegment('Active Camouflage')).toBe('active_camouflage');
		expect(slugifyTagSegment('rocket_launcher')).toBe('rocket_launcher');
	});
});

describe('iconKeyForItem (derived fallback)', () => {
	it('keys weapons by their tag folder', () => {
		expect(iconKeyForItem('weapons\\pistol\\pistol', 'weapon')).toBe('weapon_pistol');
		expect(iconKeyForItem('weapons\\rocket launcher\\rocket_launcher', 'weapon')).toBe(
			'weapon_rocket_launcher'
		);
	});

	it('detects grenades by substring regardless of classified kind', () => {
		expect(iconKeyForItem('weapons\\frag grenade\\frag grenade', 'weapon')).toBe('grenade');
		expect(iconKeyForItem('weapons\\plasma grenade\\plasma grenade', 'weapon')).toBe('grenade');
	});

	it('returns null for powerups (no derivable key — curated only)', () => {
		expect(iconKeyForItem('powerups\\over shield\\over shield', 'powerup')).toBeNull();
		expect(iconKeyForItem('characters\\cyborg\\cyborg', 'biped')).toBeNull();
	});
});

describe('resolveItemKey (curated tag_map first, then derived)', () => {
	it('maps powerups to their waypoint marker via tag_map', () => {
		expect(resolveItemKey(CE, 'powerups\\over shield\\over shield', 'powerup')).toBe(
			'marker_diamond'
		);
		expect(resolveItemKey(CE, 'powerups\\active camouflage\\active camouflage', 'powerup')).toBe(
			'marker_ring'
		);
		expect(resolveItemKey(CE, 'powerups\\health pack', 'powerup')).toBe('marker_chevron');
	});

	it('resolves weapons (tag_map and derived agree)', () => {
		expect(resolveItemKey(CE, 'weapons\\pistol\\pistol', 'weapon')).toBe('weapon_pistol');
		// a weapon absent from tag_map still derives a key (gated by the manifest later)
		expect(resolveItemKey(CE, 'weapons\\shotgun\\shotgun', 'weapon')).toBe('weapon_shotgun');
	});

	it('grenades use the tag_map wildcard', () => {
		expect(resolveItemKey(CE, 'weapons\\frag grenade\\frag grenade', 'weapon')).toBe('grenade');
	});

	it('with an empty tag_map, powerups have no key (generic fallback)', () => {
		expect(resolveItemKey(EMPTY, 'powerups\\over shield\\over shield', 'powerup')).toBeNull();
		// weapons still derive
		expect(resolveItemKey(EMPTY, 'weapons\\pistol\\pistol', 'weapon')).toBe('weapon_pistol');
	});
});

describe('resolveVehicleKey / resolveFlagKey', () => {
	it('vehicles derive a key (manifest gates existence)', () => {
		expect(resolveVehicleKey(CE, 'vehicles\\warthog\\warthog')).toBe('vehicle_warthog');
		expect(iconKeyForVehicle('vehicles\\ghost\\ghost')).toBe('vehicle_ghost');
	});

	it('flag resolves to the objective flag key', () => {
		expect(resolveFlagKey(CE)).toBe('objective_flag');
		expect(resolveFlagKey(EMPTY)).toBe(OBJECTIVE_FLAG_KEY);
	});
});

describe('emptyIconSet', () => {
	it('never resolves a url', () => {
		const s = emptyIconSet('haloce');
		expect(s.loaded).toBe(false);
		expect(s.url('weapon_pistol')).toBeNull();
		expect(s.itemUrl('powerups\\over shield\\over shield', 'powerup')).toBeNull();
		expect(s.flagUrl()).toBeNull();
	});
});

describe('loadIconSet', () => {
	const manifest = {
		game: 'haloce',
		icons: {
			weapon_pistol: { file: 'weapon_pistol.png' },
			marker_diamond: { file: 'marker_diamond.png' },
			objective_flag: { file: 'objective_flag.png' }
		},
		tag_map: {
			weapon: { pistol: 'weapon_pistol' },
			powerup: { over_shield: 'marker_diamond' },
			grenade: { '*': 'grenade' },
			objective: { flag: 'objective_flag' }
		}
	};

	it('resolves item URLs through the curated tag_map', async () => {
		const fetchFn = (async () =>
			new Response(JSON.stringify(manifest), { status: 200 })) as unknown as typeof fetch;
		const set = await loadIconSet('haloce', fetchFn);
		expect(set.loaded).toBe(true);
		// weapon via tag_map + manifest
		expect(set.itemUrl('weapons\\pistol\\pistol', 'weapon')).toBe(
			'/game-icons/haloce/weapon_pistol.png'
		);
		// powerup → waypoint marker (the whole point of point #1)
		expect(set.itemUrl('powerups\\over shield\\over shield', 'powerup')).toBe(
			'/game-icons/haloce/marker_diamond.png'
		);
		expect(set.flagUrl()).toBe('/game-icons/haloce/objective_flag.png');
		// a powerup with no mapping → null (generic fallback)
		expect(set.itemUrl('powerups\\active camouflage', 'powerup')).toBeNull();
		// vehicles ship no icon in CE → null
		expect(set.vehicleUrl('vehicles\\warthog\\warthog')).toBeNull();
	});

	it('falls back to an empty set when the cache is missing (404)', async () => {
		const fetchFn = (async () => new Response('nope', { status: 404 })) as unknown as typeof fetch;
		const set = await loadIconSet('haloce', fetchFn);
		expect(set.loaded).toBe(false);
		expect(set.itemUrl('weapons\\pistol\\pistol', 'weapon')).toBeNull();
	});

	it('falls back to an empty set when fetch throws (offline)', async () => {
		const fetchFn = (async () => {
			throw new Error('network down');
		}) as unknown as typeof fetch;
		const set = await loadIconSet('haloce', fetchFn);
		expect(set.loaded).toBe(false);
	});
});
