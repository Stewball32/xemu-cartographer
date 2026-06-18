import { describe, it, expect } from 'vitest';
import {
	projectToMinimap,
	aimToScreenAngle,
	heightBand,
	iconScale,
	transformForMap,
	type MapTransform
} from './minimap';

const base: MapTransform = {
	scenario: 'test',
	width: 200,
	height: 100,
	scale: 1,
	originX: 0,
	originY: 0,
	rotation: 0,
	flipY: false
};

describe('projectToMinimap', () => {
	it('maps the origin to the viewport center', () => {
		const p = projectToMinimap({ x: 0, y: 0, z: 0 }, base);
		expect(p).toEqual({ x: 100, y: 50 });
	});

	it('applies scale and offset from origin', () => {
		const p = projectToMinimap({ x: 10, y: 5, z: 0 }, { ...base, scale: 2 });
		expect(p).toEqual({ x: 100 + 20, y: 50 + 10 });
	});

	it('flips Y so world +Y renders upward', () => {
		const p = projectToMinimap({ x: 0, y: 10, z: 0 }, { ...base, flipY: true });
		expect(p.y).toBe(50 - 10);
	});

	it('rotates around the origin', () => {
		// 90° rotation sends world +X to screen +Y (before flip).
		const p = projectToMinimap({ x: 10, y: 0, z: 0 }, { ...base, rotation: Math.PI / 2 });
		expect(p.x).toBeCloseTo(100, 6);
		expect(p.y).toBeCloseTo(50 + 10, 6);
	});
});

describe('aimToScreenAngle', () => {
	it('passes facing through with no transform', () => {
		expect(aimToScreenAngle(1.2, base)).toBeCloseTo(1.2, 6);
	});
	it('adds the transform rotation', () => {
		expect(aimToScreenAngle(1, { ...base, rotation: 0.5 })).toBeCloseTo(1.5, 6);
	});
	it('negates when Y is flipped', () => {
		expect(aimToScreenAngle(1, { ...base, flipY: true })).toBeCloseTo(-1, 6);
	});
});

describe('heightBand', () => {
	it('returns 0 for degenerate input', () => {
		expect(heightBand(5, 0, 0, 3)).toBe(0);
		expect(heightBand(5, 0, 10, 1)).toBe(0);
	});
	it('buckets within range and clamps the ends', () => {
		expect(heightBand(0, 0, 9, 3)).toBe(0);
		expect(heightBand(4, 0, 9, 3)).toBe(1);
		expect(heightBand(9, 0, 9, 3)).toBe(2); // top clamps into the last band
		expect(heightBand(-5, 0, 9, 3)).toBe(0);
		expect(heightBand(100, 0, 9, 3)).toBe(2);
	});
});

describe('iconScale', () => {
	it('returns 1 for degenerate range', () => {
		expect(iconScale(5, 0, 0)).toBe(1);
	});
	it('interpolates and clamps to [min,max]', () => {
		expect(iconScale(0, 0, 10)).toBeCloseTo(0.7, 6);
		expect(iconScale(10, 0, 10)).toBeCloseTo(1.3, 6);
		expect(iconScale(5, 0, 10)).toBeCloseTo(1.0, 6);
		expect(iconScale(-100, 0, 10)).toBeCloseTo(0.7, 6);
		expect(iconScale(100, 0, 10)).toBeCloseTo(1.3, 6);
	});
});

describe('transformForMap', () => {
	it('looks up case-insensitively', () => {
		expect(transformForMap('Blood Gulch')?.scenario).toBe('blood gulch');
		expect(transformForMap('  blood gulch  ')?.scenario).toBe('blood gulch');
	});
	it('returns null for untraced maps', () => {
		expect(transformForMap('hang em high')).toBeNull();
		expect(transformForMap('')).toBeNull();
	});
});
