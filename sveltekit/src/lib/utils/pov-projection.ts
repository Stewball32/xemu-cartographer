// M12 POV-marker projection math (stretch milestone). A pinhole-camera
// world→screen projection so a browser-source overlay layered over a machine's
// game POV can anchor markers (enemy silhouettes, teammate tags, powerup
// labels) to world positions. This is the hard, must-be-correct part of 12a/12b
// and the only part that can be unit-tested; the live alignment pass (12e),
// split-screen partitioning (12c), marker rendering (12d), and — crucially —
// confirming the Halo: CE reader actually exposes a usable camera matrix
// (12a; spec flags missing offsets as an M19 follow-up) all need live data and
// are deferred.

import type { Vec3 } from '$lib/types/scraper-v2';

/** Per-player camera state for one tick. forward/up should be unit vectors. */
export interface CameraState {
	pos: Vec3;
	forward: Vec3;
	up: Vec3;
	/** Vertical field of view, radians. */
	fovY: number;
	/** Viewport aspect ratio (width / height). */
	aspect: number;
}

export interface ScreenPoint {
	/** Pixel coordinates in the viewport (origin top-left, y down). */
	x: number;
	y: number;
	/** Distance in front of the camera along `forward`. <= 0 means behind. */
	depth: number;
	/** True when in front of the camera and inside the viewport frustum. */
	onScreen: boolean;
}

function dot(a: Vec3, b: Vec3): number {
	return a.x * b.x + a.y * b.y + a.z * b.z;
}

function sub(a: Vec3, b: Vec3): Vec3 {
	return { x: a.x - b.x, y: a.y - b.y, z: a.z - b.z };
}

function cross(a: Vec3, b: Vec3): Vec3 {
	return {
		x: a.y * b.z - a.z * b.y,
		y: a.z * b.x - a.x * b.z,
		z: a.x * b.y - a.y * b.x
	};
}

function normalize(v: Vec3): Vec3 {
	const len = Math.hypot(v.x, v.y, v.z);
	if (len === 0) return { x: 0, y: 0, z: 0 };
	return { x: v.x / len, y: v.y / len, z: v.z / len };
}

/**
 * Project a world point to screen pixels for a `width`×`height` viewport using
 * a right-handed pinhole camera. Returns `onScreen: false` (and clamped-ish
 * coords) for points behind the camera or outside the frustum, so callers can
 * cull markers cheaply. Through-wall occlusion is explicitly out of scope for
 * v1 (needs BSP geometry from M11a's deferred case).
 */
export function worldToScreen(
	world: Vec3,
	cam: CameraState,
	width: number,
	height: number
): ScreenPoint {
	const forward = normalize(cam.forward);
	// Right-handed basis: right = forward × up, trueUp = right × forward.
	const right = normalize(cross(forward, cam.up));
	const trueUp = cross(right, forward);

	const rel = sub(world, cam.pos);
	const depth = dot(rel, forward);
	if (depth <= 0) {
		return { x: width / 2, y: height / 2, depth, onScreen: false };
	}

	const xView = dot(rel, right);
	const yView = dot(rel, trueUp);

	const tanY = Math.tan(cam.fovY / 2);
	const tanX = tanY * cam.aspect;

	const ndcX = xView / depth / tanX;
	const ndcY = yView / depth / tanY;

	const x = (ndcX * 0.5 + 0.5) * width;
	const y = (0.5 - ndcY * 0.5) * height; // screen y grows downward

	const onScreen = Math.abs(ndcX) <= 1 && Math.abs(ndcY) <= 1;
	return { x, y, depth, onScreen };
}
