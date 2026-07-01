// Halo 2 emblem BACKGROUND plates (32) — the REAL in-game set, extracted from
// the stock Xbox Halo 2 cache (`ui\global_bitmaps\emblems\background`, 128x128,
// Xbox-swizzled) by tools/h2-emblems/extract_emblems.py and shipped as a
// PRIMARY coverage mask under static/emblems/bg/<NN>_p.png.
//
// A plate is opaque and two-tone: it fills with the secondary ARMOR colour and
// overlays the PRIMARY armor colour through the mask. "solid" = full primary;
// splits = 50/50; gradients interpolate (the mask carries the coverage). The
// per-shape geometry comes straight from the bitmap, so it matches the game.
//
// IP: real game art, scoped to personal / LAN use with your own copy.

import { EMBLEM_ASSET_BASE, plate } from './emblem-art';

export const BACKGROUND_COUNT = 32;

/**
 * INNER svg for background `index`. `a` = primary armor colour (0x118), `b` =
 * secondary (0x119). `uid` makes the internal mask id unique when many render
 * at once; callers rendering more than one per page MUST pass distinct uids.
 */
export function backgroundSvg(index: number, a: string, b: string, uid?: string): string {
	if (!Number.isInteger(index) || index < 0 || index >= BACKGROUND_COUNT) {
		return `<rect x="0" y="0" width="100" height="100" fill="${a}"/>`;
	}
	const i = String(index).padStart(2, '0');
	// base = secondary (b), overlaid by primary (a) through the coverage mask.
	return plate(uid ?? `bg${index}`, `${EMBLEM_ASSET_BASE}/bg/${i}_p.png`, a, b);
}
