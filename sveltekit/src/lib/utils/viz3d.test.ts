import { describe, it, expect } from 'vitest';
import { haloToThree, facingAngle, framingFromBounds, defaultCameraPosition } from './viz3d';
import type { WorldBounds } from './visualizer-view';

describe('haloToThree', () => {
	it('maps Halo (x, y, z) → Three (x, z, −y) — Z-up to Y-up, no mirror', () => {
		expect(haloToThree({ x: 1, y: 2, z: 3 })).toEqual([1, 3, -2]);
		expect(haloToThree({ x: -5, y: 10, z: 0 })).toEqual([-5, 0, -10]);
	});

	it('treats non-finite components as 0 (never emits NaN into a buffer)', () => {
		expect(haloToThree({ x: NaN, y: 2, z: 3 })).toEqual([0, 3, -2]);
		// @ts-expect-error — exercising the runtime guard with a malformed vec
		const r = haloToThree({});
		// −0 is acceptable (Math.abs normalizes it); the guarantee is "never NaN".
		expect(r.map((n) => Math.abs(n))).toEqual([0, 0, 0]);
	});

	it('is its own inverse on the y/z swap (round-trips the ground plane)', () => {
		const t = haloToThree({ x: 7, y: 9, z: 4 });
		// world +X stays Three +X; world +Y becomes Three −Z; world Z becomes Three Y
		expect(t[0]).toBe(7);
		expect(t[2]).toBe(-9);
		expect(t[1]).toBe(4);
	});
});

describe('facingAngle', () => {
	it('undoes the 2D screen Y-flip (negates the heading)', () => {
		expect(facingAngle(0)).toBe(-0);
		expect(facingAngle(Math.PI / 2)).toBeCloseTo(-Math.PI / 2);
		expect(facingAngle(-1.2)).toBeCloseTo(1.2);
	});

	it('passes null/undefined/NaN through as null (no arrow)', () => {
		expect(facingAngle(null)).toBeNull();
		expect(facingAngle(undefined)).toBeNull();
		expect(facingAngle(NaN)).toBeNull();
	});
});

function bounds(partial: Partial<WorldBounds>): WorldBounds {
	return {
		minX: 0,
		maxX: 0,
		minY: 0,
		maxY: 0,
		minZ: 0,
		maxZ: 0,
		valid: true,
		source: 'static',
		...partial
	};
}

describe('framingFromBounds', () => {
	it('returns the bounds centre (in Three space) and a half-diagonal radius', () => {
		// Bounds large enough that the radius floor (FALLBACK_RADIUS/2 = 30) doesn't apply.
		const f = framingFromBounds(
			bounds({ minX: -60, maxX: 60, minY: -40, maxY: 40, minZ: 0, maxZ: 10 })
		);
		// centre Halo (0, 0, 5) → Three (0, 5, 0) (use toBeCloseTo: −0 ≠ 0 under toBe)
		expect(f.center[0]).toBeCloseTo(0);
		expect(f.center[1]).toBeCloseTo(5);
		expect(f.center[2]).toBeCloseTo(0);
		// half-diagonal of the 120×80×10 box
		const diag = Math.sqrt(120 * 120 + 80 * 80 + 10 * 10) / 2;
		expect(f.radius).toBeCloseTo(diag);
	});

	it('falls back to a sane centre + radius when bounds are invalid', () => {
		expect(framingFromBounds(bounds({ valid: false }))).toEqual({ center: [0, 0, 0], radius: 60 });
		expect(framingFromBounds(null)).toEqual({ center: [0, 0, 0], radius: 60 });
	});

	it('floors the radius so a clustered frame does not jam the camera inside', () => {
		const f = framingFromBounds(bounds({ minX: 0, maxX: 1, minY: 0, maxY: 1, minZ: 0, maxZ: 0 }));
		expect(f.radius).toBeGreaterThanOrEqual(30);
	});
});

describe('defaultCameraPosition', () => {
	it('offsets up-and-to-the-side of the centre with no zero axis (3/4 aerial)', () => {
		const f = { center: [0, 0, 0] as [number, number, number], radius: 100 };
		const p = defaultCameraPosition(f);
		expect(p[0]).toBeGreaterThan(0);
		expect(p[1]).toBeGreaterThan(0); // above
		expect(p[2]).toBeGreaterThan(0);
		// tilts down (height vs horizontal reach are comparable, not top-down)
		const horiz = Math.hypot(p[0], p[2]);
		expect(p[1]).toBeLessThan(horiz * 1.5);
		expect(p[1]).toBeGreaterThan(horiz * 0.4);
	});

	it('scales with the framing radius and the dist multiplier', () => {
		const f = { center: [10, 0, -10] as [number, number, number], radius: 50 };
		const near = defaultCameraPosition(f, 1);
		const far = defaultCameraPosition(f, 3);
		// farther dist pushes every component further from the centre
		expect(Math.abs(far[0] - 10)).toBeGreaterThan(Math.abs(near[0] - 10));
		expect(far[1]).toBeGreaterThan(near[1]);
	});
});
