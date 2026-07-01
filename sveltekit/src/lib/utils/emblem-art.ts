/**
 * Shared helpers for compositing the REAL extracted Halo 2 emblem masks into
 * tintable inner-SVG. One audited place that builds `<mask><image/></mask>` +
 * filled `<rect>`s, used by both emblem-foregrounds.ts and emblem-backgrounds.ts.
 *
 * The masks are grayscale luminance masks (white = full coverage) under the
 * SvelteKit static root. `maskUnits="userSpaceOnUse"` with an explicit
 * 0,0,100,100 region keeps the mask aligned to the 100x100 viewBox even when a
 * caller wraps the result in a scaled <g> (the emblem preview insets the symbol).
 */

// Emblem masks live at <base>/emblems/{fg,bg}/<NN>_{p,s}.png. Served from
// SvelteKit `static/`. If the app is mounted under a base path, prefix it here.
export const EMBLEM_ASSET_BASE = '/emblems';

function maskTag(id: string, href: string): string {
	return (
		`<mask id="${id}" maskUnits="userSpaceOnUse" x="0" y="0" width="100" height="100">` +
		`<image href="${href}" x="0" y="0" width="100" height="100" preserveAspectRatio="none"/>` +
		`</mask>`
	);
}

const fillRect = (color: string, mask?: string) =>
	`<rect x="0" y="0" width="100" height="100" fill="${color}"${mask ? ` mask="url(#${mask})"` : ''}/>`;

/**
 * Two-tone symbol: primary mask filled `primary`, secondary mask filled
 * `secondary`. No base fill, so anything outside both masks stays transparent
 * (foreground symbols). `uid` must be unique per on-page instance.
 */
export function two(
	uid: string,
	primaryMask: string,
	secondaryMask: string,
	primary: string,
	secondary: string
): string {
	const p = `${uid}-p`,
		s = `${uid}-s`;
	return (
		maskTag(s, secondaryMask) +
		maskTag(p, primaryMask) +
		fillRect(secondary, s) +
		fillRect(primary, p)
	);
}

/**
 * Opaque two-tone plate: a solid `base` (secondary) fill with the primary
 * region overlaid in `primary` via a single coverage mask. Used for background
 * plates, which fill the whole square and need only the primary mask (splits
 * and gradients fall out of the mask's coverage values).
 */
export function plate(uid: string, primaryMask: string, primary: string, base: string): string {
	const p = `${uid}-p`;
	return fillRect(base) + maskTag(p, primaryMask) + fillRect(primary, p);
}
