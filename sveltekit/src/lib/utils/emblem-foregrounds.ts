/**
 * Halo-2 emblem FOREGROUND symbols as inline SVG.
 *
 * These are ORIGINAL, hand-drawn silhouette recreations — clean stand-ins
 * inspired by the classic 64 Halo 2 foreground marks. They are intentionally
 * NOT copied from any game asset (IP-safe). Pure code, no external fetches.
 *
 * Each builder returns INNER svg markup only (no <svg> wrapper / no <?xml>),
 * designed for viewBox "0 0 100 100" and visually centered in the 14..86 box.
 * `fg` is the primary fill; `fg2` an optional secondary detail color.
 */

export const FOREGROUND_COUNT = 64;

export interface ForegroundMeta {
	index: number;
	key: string;
	label: string;
}

/** Ordered list, index 0..63. */
export const FOREGROUNDS: ForegroundMeta[] = [
	{ index: 0, key: 'seventh_column', label: 'Seventh Column' },
	{ index: 1, key: 'bullseye', label: 'Bullseye' },
	{ index: 2, key: 'vortex', label: 'Vortex' },
	{ index: 3, key: 'halt', label: 'Halt' },
	{ index: 4, key: 'spartan', label: 'Spartan' },
	{ index: 5, key: 'da_bomb', label: 'Da Bomb' },
	{ index: 6, key: 'trinity', label: 'Trinity' },
	{ index: 7, key: 'delta', label: 'Delta' },
	{ index: 8, key: 'rampancy', label: 'Rampancy' },
	{ index: 9, key: 'sergeant', label: 'Sergeant' },
	{ index: 10, key: 'phenoix', label: 'Phoenix' },
	{ index: 11, key: 'champion', label: 'Champion' },
	{ index: 12, key: 'jolly_roger', label: 'Jolly Roger' },
	{ index: 13, key: 'marathon', label: 'Marathon' },
	{ index: 14, key: 'cube', label: 'Cube' },
	{ index: 15, key: 'radioactive', label: 'Radioactive' },
	{ index: 16, key: 'smiley', label: 'Smiley' },
	{ index: 17, key: 'frowney', label: 'Frowney' },
	{ index: 18, key: 'spearhead', label: 'Spearhead' },
	{ index: 19, key: 'sol', label: 'Sol' },
	{ index: 20, key: 'waypoint', label: 'Waypoint' },
	{ index: 21, key: 'ying_yang', label: 'Yin-Yang' },
	{ index: 22, key: 'helmet', label: 'Helmet' },
	{ index: 23, key: 'triad', label: 'Triad' },
	{ index: 24, key: 'grunt_symbol', label: 'Grunt Symbol' },
	{ index: 25, key: 'cleave', label: 'Cleave' },
	{ index: 26, key: 'thor', label: 'Thor' },
	{ index: 27, key: 'skull_king', label: 'Skull King' },
	{ index: 28, key: 'triplicate', label: 'Triplicate' },
	{ index: 29, key: 'subnova', label: 'Subnova' },
	{ index: 30, key: 'flaming_ninja', label: 'Flaming Ninja' },
	{ index: 31, key: 'doubleCresent', label: 'Double Crescent' },
	{ index: 32, key: 'spades', label: 'Spades' },
	{ index: 33, key: 'clubs', label: 'Clubs' },
	{ index: 34, key: 'diamonds', label: 'Diamonds' },
	{ index: 35, key: 'hearts', label: 'Hearts' },
	{ index: 36, key: 'wasp', label: 'Wasp' },
	{ index: 37, key: 'mark_of_shame', label: 'Mark of Shame' },
	{ index: 38, key: 'snake', label: 'Snake' },
	{ index: 39, key: 'hawk', label: 'Hawk' },
	{ index: 40, key: 'lips', label: 'Lips' },
	{ index: 41, key: 'capsule', label: 'Capsule' },
	{ index: 42, key: 'cancel', label: 'Cancel' },
	{ index: 43, key: 'gas_mask', label: 'Gas Mask' },
	{ index: 44, key: 'grenade', label: 'Grenade' },
	{ index: 45, key: 'tsanta', label: 'Santa' },
	{ index: 46, key: 'race', label: 'Race' },
	{ index: 47, key: 'valkyire', label: 'Valkyrie' },
	{ index: 48, key: 'drone', label: 'Drone' },
	{ index: 49, key: 'grunt', label: 'Grunt' },
	{ index: 50, key: 'grunt_head', label: 'Grunt Head' },
	{ index: 51, key: 'brute_head', label: 'Brute Head' },
	{ index: 52, key: 'runes', label: 'Runes' },
	{ index: 53, key: 'trident', label: 'Trident' },
	{ index: 54, key: 'number0', label: '0' },
	{ index: 55, key: 'number1', label: '1' },
	{ index: 56, key: 'number2', label: '2' },
	{ index: 57, key: 'number3', label: '3' },
	{ index: 58, key: 'number4', label: '4' },
	{ index: 59, key: 'number5', label: '5' },
	{ index: 60, key: 'number6', label: '6' },
	{ index: 61, key: 'number7', label: '7' },
	{ index: 62, key: 'number8', label: '8' },
	{ index: 63, key: 'number9', label: '9' }
];

/** Builds a bold digit glyph using a <text> element. */
function digitGlyph(d: string, fg: string): string {
	return (
		`<text x="50" y="50" text-anchor="middle" dominant-baseline="central" ` +
		`font-family="Arial, sans-serif" font-weight="bold" font-size="78" fill="${fg}">${d}</text>`
	);
}

/** Lookup of glyph builders by index. */
const builders: Array<(fg: string, fg2: string) => string> = [
	// 0 seventh_column — two interlocking angular hooks
	(fg) =>
		`<path d="M30 18 H58 V32 H44 V48 H58 V62 H30 V48 H44 V32 H30 Z" fill="${fg}"/>` +
		`<path d="M70 82 H42 V68 H56 V52 H42 V38 H70 V52 H56 V68 H70 Z" fill="${fg}"/>`,

	// 1 bullseye — concentric ring target (solid silhouette via fill-rule)
	(fg) =>
		`<path d="M50 16 A34 34 0 1 1 50 84 A34 34 0 1 1 50 16 Z ` +
		`M50 27 A23 23 0 1 0 50 73 A23 23 0 1 0 50 27 Z" fill="${fg}" fill-rule="evenodd"/>` +
		`<path d="M50 34 A16 16 0 1 1 50 66 A16 16 0 1 1 50 34 Z ` +
		`M50 43 A7 7 0 1 0 50 57 A7 7 0 1 0 50 43 Z" fill="${fg}" fill-rule="evenodd"/>` +
		`<circle cx="50" cy="50" r="5" fill="${fg}"/>`,

	// 2 vortex — spiral arm
	(fg) =>
		`<path d="M50 50 m0 -32 a32 32 0 1 1 -22.6 9.4 a23 23 0 1 0 16.3 -6.8 ` +
		`a14 14 0 1 1 -9.9 4.1 a6 6 0 1 0 4.2 -1.7 l-3 9 a13 13 0 1 1 9 -3.7 ` +
		`a22 22 0 1 0 -15.5 6.4 a31 31 0 1 1 21.9 -9 Z" fill="${fg}"/>`,

	// 3 halt — open palm / stop hand
	(fg) =>
		`<rect x="34" y="44" width="32" height="38" rx="6" fill="${fg}"/>` +
		`<rect x="34" y="30" width="7" height="22" rx="3.5" fill="${fg}"/>` +
		`<rect x="43" y="22" width="7" height="30" rx="3.5" fill="${fg}"/>` +
		`<rect x="52" y="22" width="7" height="30" rx="3.5" fill="${fg}"/>` +
		`<rect x="61" y="28" width="7" height="24" rx="3.5" fill="${fg}"/>` +
		`<rect x="26" y="48" width="11" height="22" rx="5" fill="${fg}"/>`,

	// 4 spartan — Mjolnir helmet bust facing forward
	(fg) =>
		`<path d="M50 16 C34 16 26 28 26 44 C26 56 30 64 34 70 L66 70 ` +
		`C70 64 74 56 74 44 C74 28 66 16 50 16 Z" fill="${fg}"/>` +
		`<path d="M40 38 L60 38 L56 56 L44 56 Z" fill="#000" fill-opacity="0.55"/>` +
		`<rect x="46" y="34" width="8" height="34" fill="#000" fill-opacity="0.55"/>` +
		`<path d="M34 70 L66 70 L62 82 L38 82 Z" fill="${fg}"/>`,

	// 5 da_bomb — round cartoon bomb with lit fuse
	(fg, fg2) =>
		`<circle cx="46" cy="60" r="26" fill="${fg}"/>` +
		`<rect x="40" y="28" width="12" height="10" fill="${fg}"/>` +
		`<path d="M52 30 C64 22 60 14 70 14" fill="none" stroke="${fg}" stroke-width="5" stroke-linecap="round"/>` +
		`<circle cx="72" cy="13" r="6" fill="${fg2}"/>`,

	// 6 trinity — three interlocking rings (triquetra-like)
	(fg) =>
		`<circle cx="50" cy="32" r="17" fill="none" stroke="${fg}" stroke-width="8"/>` +
		`<circle cx="35" cy="60" r="17" fill="none" stroke="${fg}" stroke-width="8"/>` +
		`<circle cx="65" cy="60" r="17" fill="none" stroke="${fg}" stroke-width="8"/>`,

	// 7 delta — upward triangle
	(fg) => `<polygon points="50,18 80,80 20,80" fill="${fg}"/>`,

	// 8 rampancy — jagged angular burst
	(fg) =>
		`<polygon points="50,14 58,38 78,30 64,48 86,56 62,60 70,84 50,66 30,84 ` +
		`38,60 14,56 36,48 22,30 42,38" fill="${fg}"/>`,

	// 9 sergeant — chevron stripes
	(fg) =>
		`<polygon points="50,24 78,52 66,52 50,36 34,52 22,52" fill="${fg}"/>` +
		`<polygon points="50,44 78,72 66,72 50,56 34,72 22,72" fill="${fg}"/>`,

	// 10 phenoix — rising bird with spread wings
	(fg) =>
		`<path d="M50 24 C56 30 58 38 58 46 C72 36 84 36 84 36 C76 46 66 52 60 54 ` +
		`L60 78 L40 78 L40 54 C34 52 24 46 16 36 C16 36 28 36 42 46 ` +
		`C42 38 44 30 50 24 Z" fill="${fg}"/>`,

	// 11 champion — laurel wreath
	(fg) =>
		`<path d="M50 82 C30 78 22 60 26 36 C34 44 38 52 40 60 C42 50 46 42 50 36 ` +
		`C54 42 58 50 60 60 C62 52 66 44 74 36 C78 60 70 78 50 82 Z" fill="${fg}"/>` +
		`<circle cx="50" cy="28" r="7" fill="${fg}"/>`,

	// 12 jolly_roger — skull and crossbones
	(fg) =>
		`<path d="M50 16 C34 16 26 28 26 42 C26 52 32 58 36 62 L36 70 L64 70 L64 62 ` +
		`C68 58 74 52 74 42 C74 28 66 16 50 16 Z" fill="${fg}"/>` +
		`<circle cx="40" cy="42" r="6" fill="#000"/>` +
		`<circle cx="60" cy="42" r="6" fill="#000"/>` +
		`<polygon points="50,52 54,62 46,62" fill="#000"/>` +
		`<rect x="20" y="74" width="60" height="7" rx="3.5" fill="${fg}" transform="rotate(20 50 77)"/>` +
		`<rect x="20" y="74" width="60" height="7" rx="3.5" fill="${fg}" transform="rotate(-20 50 77)"/>`,

	// 13 marathon — stylized lightning column
	(fg) => `<polygon points="56,14 36,52 48,52 42,86 66,44 52,44 60,14" fill="${fg}"/>`,

	// 14 cube — isometric cube
	(fg) =>
		`<polygon points="50,18 80,34 50,50 20,34" fill="${fg}"/>` +
		`<polygon points="20,34 50,50 50,82 20,66" fill="${fg}" fill-opacity="0.7"/>` +
		`<polygon points="80,34 50,50 50,82 80,66" fill="${fg}" fill-opacity="0.45"/>`,

	// 15 radioactive — trefoil
	(fg) =>
		`<circle cx="50" cy="50" r="9" fill="${fg}"/>` +
		`<path d="M50 50 L70.8 14 A42 42 0 0 0 29.2 14 Z" fill="${fg}"/>` +
		`<path d="M50 50 L86 71 A42 42 0 0 0 70.8 14 Z" fill="${fg}" transform="rotate(120 50 50)"/>` +
		`<path d="M50 50 L86 71 A42 42 0 0 0 70.8 14 Z" fill="${fg}" transform="rotate(240 50 50)"/>`,

	// 16 smiley — happy face
	(fg) =>
		`<circle cx="50" cy="50" r="34" fill="${fg}"/>` +
		`<circle cx="38" cy="42" r="5.5" fill="#000"/>` +
		`<circle cx="62" cy="42" r="5.5" fill="#000"/>` +
		`<path d="M34 58 Q50 74 66 58" fill="none" stroke="#000" stroke-width="6" stroke-linecap="round"/>`,

	// 17 frowney — sad face
	(fg) =>
		`<circle cx="50" cy="50" r="34" fill="${fg}"/>` +
		`<circle cx="38" cy="44" r="5.5" fill="#000"/>` +
		`<circle cx="62" cy="44" r="5.5" fill="#000"/>` +
		`<path d="M34 68 Q50 52 66 68" fill="none" stroke="#000" stroke-width="6" stroke-linecap="round"/>`,

	// 18 spearhead — spear/arrow point up
	(fg) => `<polygon points="50,14 70,46 58,46 58,86 42,86 42,46 30,46" fill="${fg}"/>`,

	// 19 sol — sun with rays
	(fg) =>
		`<circle cx="50" cy="50" r="20" fill="${fg}"/>` +
		`<g fill="${fg}">` +
		`<polygon points="50,8 55,24 45,24"/><polygon points="50,92 55,76 45,76"/>` +
		`<polygon points="8,50 24,45 24,55"/><polygon points="92,50 76,45 76,55"/>` +
		`<polygon points="20,20 33,28 28,33"/><polygon points="80,20 72,33 67,28"/>` +
		`<polygon points="20,80 28,67 33,72"/><polygon points="80,80 67,72 72,67"/>` +
		`</g>`,

	// 20 waypoint — downward location pin / marker
	(fg) =>
		`<path d="M50 84 C40 64 26 54 26 40 A24 24 0 0 1 74 40 C74 54 60 64 50 84 Z" fill="${fg}"/>` +
		`<circle cx="50" cy="40" r="9" fill="#000"/>`,

	// 21 ying_yang — taijitu using fg + fg2
	(fg, fg2) =>
		`<circle cx="50" cy="50" r="34" fill="${fg2}"/>` +
		`<path d="M50 16 A34 34 0 0 1 50 84 A17 17 0 0 1 50 50 A17 17 0 0 0 50 16 Z" fill="${fg}"/>` +
		`<circle cx="50" cy="33" r="5" fill="${fg2}"/>` +
		`<circle cx="50" cy="67" r="5" fill="${fg}"/>`,

	// 22 helmet — side profile combat helmet
	(fg) =>
		`<path d="M24 52 C24 32 40 22 58 24 C72 26 80 36 80 46 L80 54 L52 54 ` +
		`L52 64 L40 64 C40 64 30 64 24 58 Z" fill="${fg}"/>` +
		`<rect x="54" y="44" width="22" height="8" rx="3" fill="#000" fill-opacity="0.55"/>`,

	// 23 triad — three dots triangle arrangement
	(fg) =>
		`<circle cx="50" cy="26" r="11" fill="${fg}"/>` +
		`<circle cx="30" cy="68" r="11" fill="${fg}"/>` +
		`<circle cx="70" cy="68" r="11" fill="${fg}"/>`,

	// 24 grunt_symbol — angular alien rune
	(fg) =>
		`<path d="M34 18 L66 18 L66 30 L46 30 L46 46 L62 46 L62 58 L46 58 L46 82 ` +
		`L34 82 Z" fill="${fg}"/>` +
		`<rect x="56" y="64" width="14" height="14" fill="${fg}"/>`,

	// 25 cleave — axe / cleaver blade
	(fg) =>
		`<rect x="46" y="20" width="8" height="62" rx="3" fill="${fg}"/>` +
		`<path d="M50 22 C66 22 80 30 80 46 C80 56 70 60 50 58 Z" fill="${fg}"/>`,

	// 26 thor — hammer Mjolnir
	(fg) =>
		`<rect x="44" y="40" width="12" height="44" rx="4" fill="${fg}"/>` +
		`<rect x="24" y="22" width="52" height="24" rx="5" fill="${fg}"/>`,

	// 27 skull_king — crowned skull
	(fg) =>
		`<path d="M30 30 L38 42 L50 30 L62 42 L70 30 L70 50 L30 50 Z" fill="${fg}"/>` +
		`<path d="M50 44 C36 44 30 54 30 64 C30 72 34 76 38 78 L38 84 L62 84 L62 78 ` +
		`C66 76 70 72 70 64 C70 54 64 44 50 44 Z" fill="${fg}"/>` +
		`<circle cx="42" cy="64" r="5" fill="#000"/>` +
		`<circle cx="58" cy="64" r="5" fill="#000"/>`,

	// 28 triplicate — three vertical bars
	(fg) =>
		`<rect x="26" y="22" width="12" height="56" rx="3" fill="${fg}"/>` +
		`<rect x="44" y="22" width="12" height="56" rx="3" fill="${fg}"/>` +
		`<rect x="62" y="22" width="12" height="56" rx="3" fill="${fg}"/>`,

	// 29 subnova — starburst
	(fg) =>
		`<polygon points="50,12 57,40 88,30 64,50 88,70 57,60 50,88 43,60 12,70 ` +
		`36,50 12,30 43,40" fill="${fg}"/>` +
		`<circle cx="50" cy="50" r="9" fill="${fg}"/>`,

	// 30 flaming_ninja — masked ninja head
	(fg) =>
		`<path d="M28 46 C28 30 40 22 50 22 C60 22 72 30 72 46 C72 54 68 60 60 62 ` +
		`L40 62 C32 60 28 54 28 46 Z" fill="${fg}"/>` +
		`<rect x="30" y="44" width="40" height="9" fill="#000" fill-opacity="0.6"/>` +
		`<path d="M40 62 L60 62 L56 80 L44 80 Z" fill="${fg}"/>`,

	// 31 doubleCresent — two opposed crescent moons
	(fg) =>
		`<path d="M44 18 A34 34 0 1 0 44 82 A26 26 0 1 1 44 18 Z" fill="${fg}"/>` +
		`<path d="M56 18 A34 34 0 1 1 56 82 A26 26 0 1 0 56 18 Z" fill="${fg}"/>`,

	// 32 spades — card suit
	(fg) =>
		`<path d="M50 16 C50 16 24 38 24 56 A14 14 0 0 0 46 67 ` +
		`C44 74 40 78 36 82 L64 82 C60 78 56 74 54 67 ` +
		`A14 14 0 0 0 76 56 C76 38 50 16 50 16 Z" fill="${fg}"/>`,

	// 33 clubs — card suit
	(fg) =>
		`<circle cx="50" cy="34" r="14" fill="${fg}"/>` +
		`<circle cx="34" cy="56" r="14" fill="${fg}"/>` +
		`<circle cx="66" cy="56" r="14" fill="${fg}"/>` +
		`<path d="M46 56 L54 56 L58 84 L42 84 Z" fill="${fg}"/>`,

	// 34 diamonds — card suit
	(fg) => `<polygon points="50,14 76,50 50,86 24,50" fill="${fg}"/>`,

	// 35 hearts — card suit
	(fg) =>
		`<path d="M50 82 C50 82 20 60 20 40 A16 16 0 0 1 50 32 ` +
		`A16 16 0 0 1 80 40 C80 60 50 82 50 82 Z" fill="${fg}"/>`,

	// 36 wasp — insect top view
	(fg) =>
		`<ellipse cx="50" cy="58" rx="11" ry="22" fill="${fg}"/>` +
		`<circle cx="50" cy="30" r="9" fill="${fg}"/>` +
		`<path d="M50 44 C30 36 18 44 16 50 C28 54 40 52 50 50 Z" fill="${fg}" fill-opacity="0.85"/>` +
		`<path d="M50 44 C70 36 82 44 84 50 C72 54 60 52 50 50 Z" fill="${fg}" fill-opacity="0.85"/>` +
		`<rect x="44" y="50" width="12" height="6" fill="#000" fill-opacity="0.4"/>` +
		`<rect x="44" y="62" width="12" height="6" fill="#000" fill-opacity="0.4"/>`,

	// 37 mark_of_shame — downward thumb
	(fg) =>
		`<path d="M40 56 L40 22 C40 16 50 16 50 24 L50 40 L66 40 ` +
		`C72 40 72 48 68 48 C74 48 73 56 68 56 C73 56 72 64 67 64 ` +
		`C71 64 70 72 64 72 L46 72 C42 72 40 68 40 64 Z" ` +
		`fill="${fg}" transform="rotate(180 53 47)"/>` +
		`<rect x="24" y="44" width="12" height="34" rx="3" fill="${fg}"/>`,

	// 38 snake — coiled serpent S-curve
	(fg) =>
		`<path d="M30 22 C58 22 58 50 50 50 C42 50 42 78 70 78" fill="none" ` +
		`stroke="${fg}" stroke-width="11" stroke-linecap="round"/>` +
		`<circle cx="72" cy="78" r="8" fill="${fg}"/>` +
		`<polygon points="78,76 88,74 78,80" fill="${fg}"/>`,

	// 39 hawk — bird of prey, wings swept
	(fg) =>
		`<path d="M50 30 L58 42 C72 30 86 28 86 28 C78 42 68 48 60 50 L60 56 ` +
		`L40 56 L40 50 C32 48 22 42 14 28 C14 28 28 30 42 42 L50 30 Z" fill="${fg}"/>` +
		`<polygon points="44,56 56,56 50,72" fill="${fg}"/>`,

	// 40 lips — kiss mark
	(fg) =>
		`<path d="M50 44 C44 34 30 34 26 44 C22 40 16 44 22 52 ` +
		`C30 64 44 70 50 70 C56 70 70 64 78 52 C84 44 78 40 74 44 ` +
		`C70 34 56 34 50 44 Z" fill="${fg}"/>` +
		`<path d="M28 47 H72" stroke="#000" stroke-width="2.5" stroke-opacity="0.45"/>`,

	// 41 capsule — pill capsule
	(fg, fg2) =>
		`<rect x="26" y="38" width="48" height="24" rx="12" fill="${fg}" transform="rotate(-30 50 50)"/>` +
		`<path d="M50 50 m-9 -15.6 a18 12 -30 0 1 18 0 l-18 31.2 a18 12 -30 0 1 -18 0 Z" ` +
		`fill="${fg2}"/>`,

	// 42 cancel — prohibition sign
	(fg) =>
		`<path d="M50 16 A34 34 0 1 1 50 84 A34 34 0 1 1 50 16 Z M50 28 ` +
		`A22 22 0 1 0 50 72 A22 22 0 1 0 50 28 Z" fill="${fg}" fill-rule="evenodd"/>` +
		`<rect x="44" y="22" width="12" height="56" rx="2" fill="${fg}" transform="rotate(45 50 50)"/>`,

	// 43 gas_mask — respirator with round filters
	(fg) =>
		`<path d="M32 30 H68 V52 C68 70 58 80 50 80 C42 80 32 70 32 52 Z" fill="${fg}"/>` +
		`<circle cx="40" cy="50" r="9" fill="#000" fill-opacity="0.6"/>` +
		`<circle cx="60" cy="50" r="9" fill="#000" fill-opacity="0.6"/>` +
		`<rect x="42" y="66" width="16" height="10" rx="4" fill="#000" fill-opacity="0.6"/>`,

	// 44 grenade — frag grenade
	(fg) =>
		`<rect x="36" y="22" width="28" height="8" rx="2" fill="${fg}"/>` +
		`<rect x="46" y="28" width="8" height="8" fill="${fg}"/>` +
		`<path d="M58 24 C70 22 70 34 60 34" fill="none" stroke="${fg}" stroke-width="4"/>` +
		`<rect x="32" y="36" width="36" height="44" rx="14" fill="${fg}"/>` +
		`<path d="M38 46 H62 M38 56 H62 M38 66 H62" stroke="#000" stroke-width="3" stroke-opacity="0.4"/>` +
		`<path d="M48 40 V76 M58 40 V76" stroke="#000" stroke-width="3" stroke-opacity="0.4"/>`,

	// 45 tsanta — Santa hat
	(fg, fg2) =>
		`<path d="M22 64 C30 36 56 18 78 24 C70 36 56 52 30 66 Z" fill="${fg}"/>` +
		`<rect x="18" y="62" width="60" height="12" rx="6" fill="${fg2}"/>` +
		`<circle cx="78" cy="24" r="8" fill="${fg2}"/>`,

	// 46 race — checkered flag
	(fg) =>
		`<rect x="26" y="22" width="6" height="60" fill="${fg}"/>` +
		`<g fill="${fg}">` +
		`<rect x="34" y="26" width="11" height="11"/><rect x="56" y="26" width="11" height="11"/>` +
		`<rect x="45" y="37" width="11" height="11"/><rect x="67" y="37" width="11" height="11"/>` +
		`<rect x="34" y="48" width="11" height="11"/><rect x="56" y="48" width="11" height="11"/>` +
		`</g>` +
		`<rect x="34" y="26" width="44" height="33" fill="none" stroke="${fg}" stroke-width="2"/>`,

	// 47 valkyire — winged helm
	(fg) =>
		`<path d="M38 44 C38 30 50 24 50 24 C50 24 62 30 62 44 L62 60 L38 60 Z" fill="${fg}"/>` +
		`<rect x="46" y="58" width="8" height="22" fill="${fg}"/>` +
		`<path d="M38 38 C24 32 14 34 12 36 C20 44 30 46 38 46 Z" fill="${fg}"/>` +
		`<path d="M62 38 C76 32 86 34 88 36 C80 44 70 46 62 46 Z" fill="${fg}"/>`,

	// 48 drone — flying insectoid alien
	(fg) =>
		`<ellipse cx="50" cy="54" rx="12" ry="20" fill="${fg}"/>` +
		`<circle cx="50" cy="30" r="10" fill="${fg}"/>` +
		`<circle cx="46" cy="29" r="3" fill="#000"/><circle cx="54" cy="29" r="3" fill="#000"/>` +
		`<path d="M40 46 C24 40 16 50 14 58 C28 60 40 56 44 52 Z" fill="${fg}" fill-opacity="0.8"/>` +
		`<path d="M60 46 C76 40 84 50 86 58 C72 60 60 56 56 52 Z" fill="${fg}" fill-opacity="0.8"/>`,

	// 49 grunt — small alien with methane tank
	(fg) =>
		`<path d="M40 36 C40 26 60 26 60 36 L60 46 C60 50 56 52 50 52 ` +
		`C44 52 40 50 40 46 Z" fill="${fg}"/>` +
		`<rect x="40" y="50" width="20" height="22" rx="6" fill="${fg}"/>` +
		`<rect x="58" y="38" width="14" height="30" rx="6" fill="${fg}"/>` +
		`<rect x="40" y="72" width="7" height="10" fill="${fg}"/>` +
		`<rect x="53" y="72" width="7" height="10" fill="${fg}"/>`,

	// 50 grunt_head — alien grunt mask front
	(fg) =>
		`<path d="M30 44 C30 30 42 24 50 24 C58 24 70 30 70 44 C70 56 62 62 50 62 ` +
		`C38 62 30 56 30 44 Z" fill="${fg}"/>` +
		`<circle cx="40" cy="42" r="5" fill="#000"/><circle cx="60" cy="42" r="5" fill="#000"/>` +
		`<rect x="42" y="60" width="16" height="16" rx="5" fill="${fg}"/>` +
		`<path d="M44 64 H56 M44 70 H56" stroke="#000" stroke-width="2.5" stroke-opacity="0.5"/>`,

	// 51 brute_head — gorilla-like brute head
	(fg) =>
		`<path d="M28 46 C28 30 38 22 50 22 C62 22 72 30 72 46 C72 58 64 66 56 70 ` +
		`L44 70 C36 66 28 58 28 46 Z" fill="${fg}"/>` +
		`<rect x="40" y="68" width="20" height="14" rx="4" fill="${fg}"/>` +
		`<circle cx="40" cy="44" r="4" fill="#000"/><circle cx="60" cy="44" r="4" fill="#000"/>` +
		`<path d="M44 56 Q50 60 56 56" fill="none" stroke="#000" stroke-width="3" stroke-opacity="0.55"/>`,

	// 52 runes — angular rune cluster
	(fg) =>
		`<path d="M28 20 L36 20 L36 50 L46 40 L46 50 L36 60 L36 80 L28 80 Z" fill="${fg}"/>` +
		`<path d="M52 20 L60 20 L72 38 L72 20 L80 20 L80 80 L72 80 L52 52 L52 80 L52 20 Z" ` +
		`fill="${fg}"/>` +
		`<rect x="44" y="62" width="6" height="18" fill="${fg}"/>`,

	// 53 trident — three-pronged spear
	(fg) =>
		`<rect x="46" y="40" width="8" height="44" rx="3" fill="${fg}"/>` +
		`<path d="M28 22 L28 44 M50 16 L50 44 M72 22 L72 44" stroke="${fg}" ` +
		`stroke-width="8" stroke-linecap="round"/>` +
		`<path d="M28 44 C28 50 36 52 50 52 C64 52 72 50 72 44" fill="none" ` +
		`stroke="${fg}" stroke-width="8"/>` +
		`<polygon points="28,18 24,26 32,26" fill="${fg}"/>` +
		`<polygon points="50,12 46,20 54,20" fill="${fg}"/>` +
		`<polygon points="72,18 68,26 76,26" fill="${fg}"/>`,

	// 54..63 digits 0..9
	(fg) => digitGlyph('0', fg),
	(fg) => digitGlyph('1', fg),
	(fg) => digitGlyph('2', fg),
	(fg) => digitGlyph('3', fg),
	(fg) => digitGlyph('4', fg),
	(fg) => digitGlyph('5', fg),
	(fg) => digitGlyph('6', fg),
	(fg) => digitGlyph('7', fg),
	(fg) => digitGlyph('8', fg),
	(fg) => digitGlyph('9', fg)
];

/**
 * Returns INNER svg markup for the foreground at `index`, drawn in `fg`
 * (and `fg2` for the few two-tone glyphs). Designed for viewBox "0 0 100 100".
 * Out-of-range indices yield a centered `fg` circle fallback.
 */
export function foregroundSvg(index: number, fg: string, fg2?: string): string {
	const secondary = fg2 ?? '#cfd8e3';
	if (!Number.isInteger(index) || index < 0 || index >= builders.length) {
		return `<circle cx="50" cy="50" r="26" fill="${fg}"/>`;
	}
	return builders[index](fg, secondary);
}
