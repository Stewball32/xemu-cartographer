/**
 * Shared helpers for compositing the REAL extracted Halo 2 emblem masks into
 * tintable inner-SVG. One audited place that builds `<mask><image/></mask>` +
 * filled `<rect>`s, used by both emblem-foregrounds.ts and emblem-backgrounds.ts.
 *
 * The masks are grayscale luminance masks (white = full coverage) under the
 * SvelteKit static root. `maskUnits="userSpaceOnUse"` with an explicit
 * 0,0,100,100 region keeps the mask aligned to the 100x100 viewBox even when a
 * caller wraps the result in a scaled <g> (the emblem preview insets the symbol).
 *
 * Registration: a mask normally stretches the whole source texture onto the
 * 0..100 cell. Real H2 emblem FOREGROUND bitmaps are 64x64 textures whose art is
 * anchored at the texture origin and only fills a top-left sprite rect (the rest
 * is power-of-two padding), so foreground callers pass that `SrcRect` to map the
 * sprite rect — not the padded texture — onto the cell. Without it the symbol
 * renders shrunk into the top-left corner instead of centred. Backgrounds fill
 * their whole texture and omit it.
 */

// Emblem masks live at <base>/emblems/{fg,bg}/<NN>_{p,s}.png. Served from
// SvelteKit `static/`. If the app is mounted under a base path, prefix it here.
export const EMBLEM_ASSET_BASE = '/emblems';

/**
 * Active sub-rectangle of a source bitmap (in source-texture pixels) that should
 * map onto the mask's 0..100 viewport. `texW/texH` are the full (padded) texture
 * dimensions; `x,y,w,h` are the sprite rect within it. See module header.
 */
export interface SrcRect {
	x: number;
	y: number;
	w: number;
	h: number;
	texW: number;
	texH: number;
}

const round3 = (v: number): number => Number(v.toFixed(3));

function maskTag(id: string, href: string, src?: SrcRect): string {
	let image: string;
	if (src) {
		// Scale the texture so the sprite rect (src.x,src.y,src.w,src.h) lands on
		// 0,0..100,100, then shift so its top-left sits at the viewport origin. The
		// mask only composites within 0..100, so the padded remainder is clipped.
		const sx = 100 / src.w;
		const sy = 100 / src.h;
		image =
			`<image href="${href}" x="${round3(-src.x * sx)}" y="${round3(-src.y * sy)}"` +
			` width="${round3(src.texW * sx)}" height="${round3(src.texH * sy)}" preserveAspectRatio="none"/>`;
	} else {
		image = `<image href="${href}" x="0" y="0" width="100" height="100" preserveAspectRatio="none"/>`;
	}
	return (
		`<mask id="${id}" maskUnits="userSpaceOnUse" x="0" y="0" width="100" height="100">` +
		image +
		`</mask>`
	);
}

const fillRect = (color: string, mask?: string) =>
	`<rect x="0" y="0" width="100" height="100" fill="${color}"${mask ? ` mask="url(#${mask})"` : ''}/>`;

/**
 * Two-tone symbol: primary mask filled `primary`, secondary mask filled
 * `secondary`. No base fill, so anything outside both masks stays transparent
 * (foreground symbols). `uid` must be unique per on-page instance. `src` maps the
 * bitmap's sprite rect onto the cell (foregrounds); omit to stretch the whole
 * texture.
 */
export function two(
	uid: string,
	primaryMask: string,
	secondaryMask: string,
	primary: string,
	secondary: string,
	src?: SrcRect
): string {
	const p = `${uid}-p`,
		s = `${uid}-s`;
	return (
		maskTag(s, secondaryMask, src) +
		maskTag(p, primaryMask, src) +
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
