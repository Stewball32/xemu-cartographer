import { describe, it, expect } from 'vitest';
import { worldToScreen, type CameraState } from './pov-projection';

// Camera at origin looking down +X, up is +Z.
const cam: CameraState = {
	pos: { x: 0, y: 0, z: 0 },
	forward: { x: 1, y: 0, z: 0 },
	up: { x: 0, y: 0, z: 1 },
	fovY: Math.PI / 2, // 90°
	aspect: 1
};

const W = 200;
const H = 100;

describe('worldToScreen', () => {
	it('projects a point straight ahead to the viewport center', () => {
		const p = worldToScreen({ x: 10, y: 0, z: 0 }, cam, W, H);
		expect(p.onScreen).toBe(true);
		expect(p.depth).toBeCloseTo(10, 6);
		expect(p.x).toBeCloseTo(W / 2, 6);
		expect(p.y).toBeCloseTo(H / 2, 6);
	});

	it('reports points behind the camera as off-screen', () => {
		const p = worldToScreen({ x: -10, y: 0, z: 0 }, cam, W, H);
		expect(p.onScreen).toBe(false);
		expect(p.depth).toBeLessThanOrEqual(0);
	});

	it('places a point to the camera-left correctly', () => {
		// right = forward(+X) × up(+Z) = (-? ) — verify by sign of screen x.
		// World +Y is to the camera's left for this basis, so it should land
		// left of center (x < W/2).
		const p = worldToScreen({ x: 10, y: 5, z: 0 }, cam, W, H);
		expect(p.onScreen).toBe(true);
		expect(p.x).toBeLessThan(W / 2);
	});

	it('places a higher point above center (smaller screen y)', () => {
		const p = worldToScreen({ x: 10, y: 0, z: 5 }, cam, W, H);
		expect(p.onScreen).toBe(true);
		expect(p.y).toBeLessThan(H / 2);
	});

	it('marks points outside the frustum as off-screen but keeps depth', () => {
		// Far to the side at shallow depth → outside 90° fov.
		const p = worldToScreen({ x: 1, y: 100, z: 0 }, cam, W, H);
		expect(p.depth).toBeCloseTo(1, 6);
		expect(p.onScreen).toBe(false);
	});

	it('a point just inside the fov edge lands near the viewport boundary', () => {
		// 90° fov, aspect 1 → horizontal half-angle is 45°, so xView ≈ depth is
		// the right edge. Use 9.9 (just inside) to avoid the exact-boundary
		// floating-point knife-edge.
		const p = worldToScreen({ x: 10, y: -9.9, z: 0 }, cam, W, H);
		expect(p.onScreen).toBe(true);
		expect(p.x).toBeGreaterThan(W * 0.95); // hugging the right edge
		expect(p.x).toBeLessThanOrEqual(W);
	});
});
