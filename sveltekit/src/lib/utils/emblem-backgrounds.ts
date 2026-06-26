// Halo 2 emblem BACKGROUND plates (32) — procedural geometry, index = enum order
// (e_emblem_background). Each returns INNER svg markup for a "0 0 100 100"
// viewBox, two-tone in the two ARMOR colors: `a` = primary (0x118), `b` =
// secondary (0x119). Original geometric recreations (no game assets).
//
// Gradients are rendered as discrete bands (not <linearGradient> in <defs>) so
// many emblems can render on one page without id collisions.

export const BACKGROUND_COUNT = 32;

function clampByte(h: string): string {
	return h.length === 4 ? '#' + h[1] + h[1] + h[2] + h[2] + h[3] + h[3] : h;
}

/** linear-interpolate two hex colors. */
function mix(a: string, b: string, t: number): string {
	const pa = clampByte(a),
		pb = clampByte(b);
	const ai = [1, 3, 5].map((i) => parseInt(pa.slice(i, i + 2), 16));
	const bi = [1, 3, 5].map((i) => parseInt(pb.slice(i, i + 2), 16));
	const c = ai.map((v, i) => Math.round(v + (bi[i] - v) * t));
	return '#' + c.map((v) => v.toString(16).padStart(2, '0')).join('');
}

const rect = (x: number, y: number, w: number, h: number, fill: string) =>
	`<rect x="${x}" y="${y}" width="${w}" height="${h}" fill="${fill}"/>`;
const poly = (pts: string, fill: string) => `<polygon points="${pts}" fill="${fill}"/>`;
const circ = (cx: number, cy: number, r: number, fill: string) =>
	`<circle cx="${cx}" cy="${cy}" r="${r}" fill="${fill}"/>`;

/** N horizontal (vertical=false) or vertical bands interpolating a→b. */
function gradient(a: string, b: string, vertical: boolean, n = 8): string {
	let out = '';
	const step = 100 / n;
	for (let i = 0; i < n; i++) {
		const fill = mix(a, b, i / (n - 1));
		out += vertical
			? rect(0, i * step, 100, step + 0.5, fill)
			: rect(i * step, 0, step + 0.5, 100, fill);
	}
	return out;
}

/** Background inner-SVG for an emblem-background byte. `a`=primary, `b`=secondary. */
export function backgroundSvg(index: number, a: string, b: string): string {
	const base = (fill: string) => rect(0, 0, 100, 100, fill);
	switch (index) {
		case 0: // solid
			return base(a);
		case 1: // vertical_split
			return rect(0, 0, 50, 100, a) + rect(50, 0, 50, 100, b);
		case 2: // horizontal_split1
			return rect(0, 0, 100, 50, a) + rect(0, 50, 100, 50, b);
		case 3: // horizontal_split2
			return rect(0, 0, 100, 50, b) + rect(0, 50, 100, 50, a);
		case 4: // vertical_gradient
			return gradient(a, b, true);
		case 5: // horizontal_gradient
			return gradient(a, b, false);
		case 6: // triple_column
			return (
				rect(0, 0, 33.34, 100, a) + rect(33.33, 0, 33.34, 100, b) + rect(66.66, 0, 33.34, 100, a)
			);
		case 7: // triple_row
			return (
				rect(0, 0, 100, 33.34, a) + rect(0, 33.33, 100, 33.34, b) + rect(0, 66.66, 100, 33.34, a)
			);
		case 8: // quadrants1 (checker)
			return base(a) + rect(50, 0, 50, 50, b) + rect(0, 50, 50, 50, b);
		case 9: // quadrants2 (inverse checker)
			return base(a) + rect(0, 0, 50, 50, b) + rect(50, 50, 50, 50, b);
		case 10: // diagonal_slice
			return base(a) + poly('100,0 100,100 0,100', b);
		case 11: // cleft (wedge)
			return base(a) + poly('30,0 70,0 50,62', b);
		case 12: // x1 (thin saltire)
			return (
				base(a) +
				poly('0,12 12,0 100,88 100,100 88,100 0,12', b) +
				poly('88,0 100,0 100,12 12,100 0,100 0,88', b)
			);
		case 13: // x2 (bold saltire)
			return (
				base(b) +
				poly('0,22 22,0 100,78 100,100 78,100 0,22', a) +
				poly('78,0 100,0 100,22 22,100 0,100 0,78', a)
			);
		case 14: // dircle
			return base(a) + circ(50, 50, 32, b);
		case 15: // diamond
			return base(a) + poly('50,16 84,50 50,84 16,50', b);
		case 16: // cross
			return base(a) + rect(40, 14, 20, 72, b) + rect(14, 40, 72, 20, b);
		case 17: // square
			return base(a) + rect(24, 24, 52, 52, b);
		case 18: // dual_half_circle
			return (
				base(a) +
				`<path d="M0,18 A32,32 0 0,1 0,82 Z" fill="${b}"/>` +
				`<path d="M100,18 A32,32 0 0,0 100,82 Z" fill="${b}"/>`
			);
		case 19: // triangle
			return base(a) + poly('50,18 84,82 16,82', b);
		case 20: // diagonal_quadrant
			return base(a) + poly('0,0 100,0 0,100', b);
		case 21: // three_quaters (three quadrants accented)
			return base(b) + rect(50, 50, 50, 50, a);
		case 22: // quarter
			return base(a) + rect(0, 0, 50, 50, b);
		case 23: // four_rows1
			return (
				rect(0, 0, 100, 25, a) +
				rect(0, 25, 100, 25, b) +
				rect(0, 50, 100, 25, a) +
				rect(0, 75, 100, 25, b)
			);
		case 24: // four_rows2 (offset)
			return (
				rect(0, 0, 100, 25, b) +
				rect(0, 25, 100, 25, a) +
				rect(0, 50, 100, 25, b) +
				rect(0, 75, 100, 25, a)
			);
		case 25: // split_circle
			return base(a) + circ(50, 50, 32, b) + `<path d="M50,18 A32,32 0 0,0 50,82 Z" fill="${a}"/>`;
		case 26: // one_third (bottom third)
			return base(a) + rect(0, 66.66, 100, 33.34, b);
		case 27: // two_thirds (bottom two thirds)
			return base(a) + rect(0, 33.33, 100, 66.67, b);
		case 28: // upper_field
			return base(a) + rect(0, 0, 100, 38, b);
		case 29: // top_and_bottom
			return base(a) + rect(0, 0, 100, 26, b) + rect(0, 74, 100, 26, b);
		case 30: // center_stripe
			return base(a) + rect(0, 37, 100, 26, b);
		case 31: // left_and_right
			return base(a) + rect(0, 0, 26, 100, b) + rect(74, 0, 26, 100, b);
		default:
			return base(a);
	}
}
