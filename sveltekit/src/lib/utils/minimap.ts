// M11 minimap projection math. Pure helpers that map Halo world coordinates
// onto a 2D minimap viewport, plus the height cues from 11c. The actual SVG
// floor tracings (11a) + canvas/SVG rendering + flares (11e) are the
// browser-rendered overlay surface that consumes these; keeping the math here
// (and unit-tested) isolates the part that has to be correct from the part
// that can only be eyeballed over OBS.

import type { Vec2, Vec3 } from '$lib/types/scraper-v2';

/**
 * A per-map transform from Halo world coordinates to minimap pixel space.
 * Committed alongside each map's SVG floor tracing (M11a), keyed by the
 * `map` / scenario name the reader reports. `originX/Y` is the world point
 * that lands at the viewport center; `rotation` aligns world "north" with the
 * asset; `flipY` accounts for SVG's y-grows-down axis.
 */
export interface MapTransform {
	scenario: string;
	width: number;
	height: number;
	scale: number;
	originX: number;
	originY: number;
	rotation: number;
	flipY: boolean;
}

/**
 * Project a world position onto the minimap. Order: translate to the map
 * origin, rotate, scale to pixels, then place relative to the viewport center
 * (flipping Y when the asset's vertical axis points up).
 */
export function projectToMinimap(pos: Vec3, t: MapTransform): Vec2 {
	const dx = pos.x - t.originX;
	const dy = pos.y - t.originY;

	const cos = Math.cos(t.rotation);
	const sin = Math.sin(t.rotation);
	const rx = (dx * cos - dy * sin) * t.scale;
	const ry = (dx * sin + dy * cos) * t.scale;

	const cx = t.width / 2;
	const cy = t.height / 2;
	return { x: cx + rx, y: t.flipY ? cy - ry : cy + ry };
}

/**
 * Convert a world-space facing angle (radians, the reader's `facing`) into the
 * minimap's screen-space rotation, accounting for the transform's rotation and
 * Y flip. Use it to point a player's view cone.
 */
export function aimToScreenAngle(facing: number, t: MapTransform): number {
	const rotated = facing + t.rotation;
	return t.flipY ? -rotated : rotated;
}

/**
 * Map a Z (vertical) coordinate to a 0..bands-1 band index over [zMin, zMax]
 * (11c colour-tint / Z-banded layer cue). Out-of-range Z clamps to the end
 * bands; degenerate input returns band 0.
 */
export function heightBand(z: number, zMin: number, zMax: number, bands: number): number {
	if (bands <= 1 || zMax <= zMin) return 0;
	const t = (z - zMin) / (zMax - zMin);
	const clamped = Math.max(0, Math.min(1, t));
	return Math.min(bands - 1, Math.floor(clamped * bands));
}

/**
 * Scale an icon between `min`× and `max`× based on Z within [zMin, zMax]
 * (11c icon-size cue — higher = larger / closer to camera). Clamps to the
 * range; degenerate input returns 1.
 */
export function iconScale(z: number, zMin: number, zMax: number, min = 0.7, max = 1.3): number {
	if (zMax <= zMin) return 1;
	const t = Math.max(0, Math.min(1, (z - zMin) / (zMax - zMin)));
	return min + t * (max - min);
}

// Registry of committed map transforms, keyed by scenario/map name. v1 seeds a
// single placeholder (Blood Gulch); real values are calibrated against the
// committed SVG tracing when 11a's assets land. Extend as maps are traced.
export const MAP_TRANSFORMS: Record<string, MapTransform> = {
	'blood gulch': {
		scenario: 'blood gulch',
		width: 256,
		height: 256,
		scale: 0.0035,
		originX: 0,
		originY: 0,
		rotation: 0,
		flipY: true
	}
};

/**
 * Look up the transform for a map/scenario name (case-insensitive, trimmed).
 * Returns null when the map hasn't been traced yet, so the overlay can render
 * a "minimap unavailable" state instead of projecting onto a wrong asset.
 */
export function transformForMap(scenario: string): MapTransform | null {
	if (!scenario) return null;
	return MAP_TRANSFORMS[scenario.trim().toLowerCase()] ?? null;
}
