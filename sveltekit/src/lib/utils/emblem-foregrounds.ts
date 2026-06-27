/**
 * Halo-2 emblem FOREGROUND symbols — the REAL in-game set.
 *
 * The 64 symbols are extracted from the stock Xbox Halo 2 cache
 * (`ui\global_bitmaps\emblems\foreground`, 64x64 a4r4g4b4, Xbox-swizzled) by
 * tools/h2-emblems/extract_emblems.py and shipped as two-tone coverage masks
 * under static/emblems/fg/<NN>_{p,s}.png:
 *   _p = PRIMARY coverage (the R/“yellow” marker channel)
 *   _s = SECONDARY coverage (the B/“blue” marker channel)
 * Outside the symbol both are 0 (transparent).
 *
 * `foregroundSvg` returns INNER svg markup for a "0 0 100 100" viewBox: it
 * tints the primary mask with `fg` (emblem primary) and the secondary mask
 * with `fg2` (emblem secondary), matching the in-game two-colour symbol. Pure
 * (no fetch beyond the <image> the browser loads from /static).
 *
 * IP: real game art, scoped to personal / LAN use with your own copy.
 */

export const FOREGROUND_COUNT = 64;

export interface ForegroundMeta {
	index: number;
	key: string;
	label: string;
}

/** Ordered list, index 0..63 (e_emblem_foreground enum order). */
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

// Where the extracted masks live (SvelteKit static root). Override here if the
// app is mounted under a base path.
import { EMBLEM_ASSET_BASE, two } from './emblem-art';

/**
 * INNER svg for foreground `index`, tinted with the two emblem colours.
 * `fg` = primary (emblem primary / tertiary), `fg2` = secondary (quaternary).
 * `uid` makes the internal mask ids unique when many emblems render at once;
 * callers that render more than one on a page MUST pass distinct uids.
 */
export function foregroundSvg(index: number, fg: string, fg2?: string, uid?: string): string {
	const secondary = fg2 ?? '#cfd8e3';
	if (!Number.isInteger(index) || index < 0 || index >= FOREGROUND_COUNT) {
		return `<circle cx="50" cy="50" r="26" fill="${fg}"/>`;
	}
	const i = String(index).padStart(2, '0');
	return two(
		uid ?? `fg${index}`,
		`${EMBLEM_ASSET_BASE}/fg/${i}_p.png`,
		`${EMBLEM_ASSET_BASE}/fg/${i}_s.png`,
		fg,
		secondary
		// no opaque base: foregrounds are transparent outside the symbol
	);
}
