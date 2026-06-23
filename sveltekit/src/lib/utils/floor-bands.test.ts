import { describe, it, expect } from 'vitest';
import { computeFloorBands, bandColor, shadeForZ } from './floor-bands';

function lum(rgb: string): number {
	const m = rgb.match(/rgb\((\d+),\s*(\d+),\s*(\d+)\)/);
	if (!m) return NaN;
	return Number(m[1]) + Number(m[2]) + Number(m[3]);
}

describe('computeFloorBands', () => {
	it('clusters three distinct floor levels into three contiguous bands, low→high', () => {
		const fb = computeFloorBands([0, 0, 0, 5, 5, 10], 5);
		expect(fb.count).toBe(3);
		expect(fb.bandForZ(0)).toBe(0);
		expect(fb.bandForZ(5)).toBe(1);
		expect(fb.bandForZ(10)).toBe(2);
		// Bands are ordered ascending by centroid.
		expect(fb.bands[0].midZ).toBeLessThan(fb.bands[1].midZ);
		expect(fb.bands[1].midZ).toBeLessThan(fb.bands[2].midZ);
	});

	it('never exceeds the requested band count and drops empty clusters', () => {
		const fb = computeFloorBands([1, 1, 1, 1], 5);
		expect(fb.count).toBe(1);
		expect(fb.bands).toHaveLength(1);
	});

	it('degrades safely on empty input', () => {
		const fb = computeFloorBands([], 5);
		expect(fb.count).toBe(1);
		expect(fb.bandForZ(123)).toBe(0);
	});

	it('assigns an out-of-range Z to the nearest edge band', () => {
		const fb = computeFloorBands([0, 5, 10], 5);
		expect(fb.bandForZ(-100)).toBe(0);
		expect(fb.bandForZ(100)).toBe(2);
	});
});

describe('bandColor', () => {
	it('ramps lower bands darker and higher bands lighter', () => {
		const lo = bandColor(0, 4);
		const hi = bandColor(3, 4);
		expect(lum(lo)).toBeLessThan(lum(hi));
	});
	it('returns a mid tone for a single band', () => {
		expect(bandColor(0, 1)).toMatch(/^rgb\(/);
	});
});

describe('shadeForZ', () => {
	it('absolute mode: minZ is darker than maxZ', () => {
		const lo = shadeForZ(0, { mode: 'absolute', minZ: 0, maxZ: 10 });
		const hi = shadeForZ(10, { mode: 'absolute', minZ: 0, maxZ: 10 });
		expect(lum(lo)).toBeLessThan(lum(hi));
	});

	it('relative mode: the followed Z reads as a mid tone, above lighter, below darker', () => {
		const opts = { mode: 'relative' as const, minZ: 0, maxZ: 10, focusZ: 5, relativeRange: 5 };
		const at = lum(shadeForZ(5, opts));
		const above = lum(shadeForZ(10, opts));
		const below = lum(shadeForZ(0, opts));
		expect(below).toBeLessThan(at);
		expect(at).toBeLessThan(above);
	});
});
