// Pure math for the 3D visualizer (the Three.js analogue of visualizer-view's
// 2D projection). Kept IO-free / DOM-free so the part that has to be CORRECT —
// the coordinate remap that aligns live markers with the extracted BSP mesh, and
// the camera framing — unit-tests without a browser or a GPU. Scene3D.svelte
// does only the Three.js wiring on top of these.
//
// Coordinate system. Halo world is right-handed with X/Y the horizontal ground
// plane and Z up (RUNTIME-VERIFIED biped pos at +0x0C..0x14; see visualizer-
// view.ts). Three.js is right-handed with Y up. We map every Halo position and
// direction through `haloToThree` so player/item/vehicle markers and the BSP
// mesh share one frame and line up regardless of which produced a given point.

import type { Vec3 } from '$lib/types/scraper-v2';
import type { WorldBounds } from '$lib/utils/visualizer-view';

/**
 * Halo world → Three.js. A proper −90° rotation about X: (x, y, z) → (x, z, −y).
 * This puts Halo's Z-up on Three's Y-up WITHOUT mirroring (handedness is
 * preserved, so normals / winding / text are not flipped). Returns a tuple so it
 * can feed `Vector3.set(...pt)` or a flat BufferAttribute without allocating a
 * Vector3 per call.
 */
export function haloToThree(v: Vec3): [number, number, number] {
	const x = Number.isFinite(v?.x) ? v.x : 0;
	const y = Number.isFinite(v?.y) ? v.y : 0;
	const z = Number.isFinite(v?.z) ? v.z : 0;
	return [x, z, -y];
}

/**
 * Recover the world ground-plane facing angle (radians, CCW from +X) from the
 * VizModel's `heading`. buildVizModel stores heading as atan2(−ay, ax) (the
 * screen-space angle, Y already flipped for the 2D map); undoing that flip gives
 * back the true world-plane angle atan2(ay, ax). Returns null when aim was ~zero.
 */
export function facingAngle(heading: number | null | undefined): number | null {
	return heading == null || !Number.isFinite(heading) ? null : -heading;
}

/** Camera-framing summary for a set of world bounds, in THREE space. */
export interface SceneFraming {
	/** Bounds centre, mapped to Three coordinates. */
	center: [number, number, number];
	/** Half-diagonal of the horizontal footprint — a stable orbit radius that
	 *  frames the whole map regardless of aspect. */
	radius: number;
}

/** Fallback footprint when bounds are invalid (no scenario, no entities yet) so
 *  the camera still has somewhere sane to look. */
const FALLBACK_RADIUS = 60;

/**
 * Reduce world bounds to a centre + radius for camera placement. Uses the full
 * 3D extent so tall geometry (Blood Gulch's canyon walls) is still framed, and
 * floors the radius so a single clustered frame doesn't jam the camera inside
 * the markers.
 */
export function framingFromBounds(b: WorldBounds | null | undefined): SceneFraming {
	if (!b || !b.valid) {
		return { center: [0, 0, 0], radius: FALLBACK_RADIUS };
	}
	const cx = (b.minX + b.maxX) / 2;
	const cy = (b.minY + b.maxY) / 2;
	const cz = (b.minZ + b.maxZ) / 2;
	const dx = b.maxX - b.minX;
	const dy = b.maxY - b.minY;
	const dz = b.maxZ - b.minZ;
	// Half-diagonal of the bounding box; keeps the whole map in frame.
	const diag = Math.sqrt(dx * dx + dy * dy + dz * dz) / 2;
	const radius = Math.max(FALLBACK_RADIUS / 2, diag);
	return { center: haloToThree({ x: cx, y: cy, z: cz }), radius };
}

/**
 * A pleasant default camera position for a framing: offset up-and-back from the
 * centre along a fixed compass bearing so the level reads as a 3/4 aerial view.
 * `dist` scales the orbit radius (1.0 ≈ tight, larger pulls back).
 */
export function defaultCameraPosition(f: SceneFraming, dist = 1.9): [number, number, number] {
	const r = f.radius * dist;
	// Up-and-to-the-side: components chosen so no axis is zero (avoids a
	// degenerate top-down or side-on start) and the view tilts down ~35°.
	return [f.center[0] + r * 0.62, f.center[1] + r * 0.66, f.center[2] + r * 0.62];
}
