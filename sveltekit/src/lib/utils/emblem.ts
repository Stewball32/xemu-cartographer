// Halo 2 / Halo: CE player-appearance model — the value sets + palette + the
// pure mapping between the H2 profile `appearance` byte map and a friendly
// emblem/armor state. PURE (no IO, no Svelte) so it unit-tests cleanly.
//
// PROVENANCE of the value sets (index order is authoritative — it is the in-game
// enum order): reverse-engineered from the predecessor H2 stats reader's
// `s_player_profile` / `e_player_color` / `e_emblem_foreground` /
// `e_emblem_background` / `e_character_type` enums, and cross-checked against two
// real captured H2 `profile` saves (the 0x118 appearance block decodes cleanly
// against the struct). See docs/gamertag-system/H2-EMBLEM-FORMAT.md.
//
// The 0x118 appearance block (24 bytes, in a 500-byte profile):
//   0x118 primary_color      (armor primary)      — e_player_color   0..17
//   0x119 secondary_color    (armor secondary)    — e_player_color   0..17
//   0x11A tertiary_color     (emblem primary)     — e_player_color   0..17
//   0x11B quaternary_color   (emblem secondary)   — e_player_color   0..17
//   0x11C player_character_type                    — e_character_type 0..3
//   0x11D emblem_foreground  (symbol)             — e_emblem_foreground 0..63
//   0x11E emblem_background  (shape)              — e_emblem_background 0..31
//   0x11F emblem_flags
//
// Emblem composition (matches the in-game model + the predecessor's emblem
// renderer): the BACKGROUND plate is drawn with the two ARMOR colors
// (primary/secondary); the FOREGROUND symbol is drawn with the two EMBLEM colors
// (tertiary/quaternary).

export { FOREGROUNDS, FOREGROUND_COUNT, foregroundSvg } from './emblem-foregrounds';
export type { ForegroundMeta } from './emblem-foregrounds';

export interface PaletteColor {
	name: string;
	hex: string;
}

// e_player_color (18) — the Halo 2 armor/emblem palette, in enum order.
// Hex derived from the app's `--color-armor-*` oklch tokens (layout.css), which
// are tuned to the in-game biped tints, so swatches match the rest of the UI.
export const H2_COLORS: PaletteColor[] = [
	{ name: 'White', hex: '#f0eeeb' }, // 0
	{ name: 'Steel', hex: '#69737d' }, // 1
	{ name: 'Red', hex: '#ea1f2f' }, // 2
	{ name: 'Orange', hex: '#f87300' }, // 3
	{ name: 'Gold', hex: '#e3ae28' }, // 4
	{ name: 'Olive', hex: '#757628' }, // 5
	{ name: 'Green', hex: '#00a327' }, // 6
	{ name: 'Sage', hex: '#7a9a69' }, // 7
	{ name: 'Cyan', hex: '#1ad1d1' }, // 8
	{ name: 'Teal', hex: '#00918b' }, // 9
	{ name: 'Cobalt', hex: '#003fb7' }, // 10
	{ name: 'Blue', hex: '#0070e5' }, // 11
	{ name: 'Violet', hex: '#6b2ae3' }, // 12
	{ name: 'Purple', hex: '#7a31ca' }, // 13
	{ name: 'Pink', hex: '#fe8dc5' }, // 14
	{ name: 'Crimson', hex: '#ae002b' }, // 15
	{ name: 'Brown', hex: '#6d3714' }, // 16
	{ name: 'Tan', hex: '#ccb48c' } // 17
];

// Halo: CE armor palette (18) — c20.reclaimers.net order, which matches the
// blam.sav 0x18 color enum (Red=2 / Blue=3 confirmed against real saves).
export const CE_COLORS: PaletteColor[] = [
	{ name: 'White', hex: '#f0eeeb' }, // 0
	{ name: 'Black', hex: '#1a1a1d' }, // 1
	{ name: 'Red', hex: '#ea1f2f' }, // 2
	{ name: 'Blue', hex: '#0070e5' }, // 3
	{ name: 'Gray', hex: '#84868c' }, // 4
	{ name: 'Yellow', hex: '#f3cb00' }, // 5
	{ name: 'Green', hex: '#00a327' }, // 6
	{ name: 'Pink', hex: '#fe8dc5' }, // 7
	{ name: 'Purple', hex: '#7a31ca' }, // 8
	{ name: 'Cyan', hex: '#1ad1d1' }, // 9
	{ name: 'Cobalt', hex: '#003fb7' }, // 10
	{ name: 'Orange', hex: '#f87300' }, // 11
	{ name: 'Teal', hex: '#00918b' }, // 12
	{ name: 'Sage', hex: '#7a9a69' }, // 13
	{ name: 'Brown', hex: '#6d3714' }, // 14
	{ name: 'Tan', hex: '#ccb48c' }, // 15
	{ name: 'Maroon', hex: '#850e2c' }, // 16
	{ name: 'Salmon', hex: '#eb827b' } // 17
];

export interface NamedIndex {
	index: number;
	key: string;
	label: string;
}

// e_emblem_background (32) — enum order. The geometry for each lives in
// emblem-backgrounds.ts (byte index → shape generator).
export const H2_BACKGROUNDS: NamedIndex[] = [
	'solid',
	'vertical_split',
	'horizontal_split1',
	'horizontal_split2',
	'vertical_gradient',
	'horizontal_gradient',
	'triple_column',
	'triple_row',
	'quadrants1',
	'quadrants2',
	'diagonal_slice',
	'cleft',
	'x1',
	'x2',
	'dircle',
	'diamond',
	'cross',
	'square',
	'dual_half_circle',
	'triangle',
	'diagonal_quadrant',
	'three_quaters',
	'quarter',
	'four_rows1',
	'four_rows2',
	'split_circle',
	'one_third',
	'two_thirds',
	'upper_field',
	'top_and_bottom',
	'center_stripe',
	'left_and_right'
].map((key, index) => ({ index, key, label: titleCase(key) }));

// e_character_type (4) — enum order.
export const H2_CHARACTERS: NamedIndex[] = [
	{ index: 0, key: 'masterchief', label: 'Master Chief' },
	{ index: 1, key: 'dervish', label: 'Dervish' },
	{ index: 2, key: 'spartan', label: 'Spartan' },
	{ index: 3, key: 'elite', label: 'Elite' }
];

// The H2 `appearance` byte-map keys + their absolute file offsets. These MUST
// stay in sync with internal/halosave/h2profile.go H2ProfileFields.
export const H2_KEYS = {
	armorPrimary: 'armor_primary',
	armorSecondary: 'armor_secondary',
	emblemPrimary: 'emblem_primary',
	emblemSecondary: 'emblem_secondary',
	character: 'character_type',
	foreground: 'emblem_foreground',
	background: 'emblem_background',
	flags: 'emblem_flags'
} as const;

export const H2_OFFSETS: Record<string, number> = {
	[H2_KEYS.armorPrimary]: 0x118,
	[H2_KEYS.armorSecondary]: 0x119,
	[H2_KEYS.emblemPrimary]: 0x11a,
	[H2_KEYS.emblemSecondary]: 0x11b,
	[H2_KEYS.character]: 0x11c,
	[H2_KEYS.foreground]: 0x11d,
	[H2_KEYS.background]: 0x11e,
	[H2_KEYS.flags]: 0x11f
};

/** The friendly emblem/armor state behind the byte map. */
export interface EmblemState {
	armorPrimary: number;
	armorSecondary: number;
	emblemPrimary: number;
	emblemSecondary: number;
	character: number;
	foreground: number;
	background: number;
}

// A sensible, visible default (mirrors the "Stew" sample: cobalt/blue armor,
// white Jolly Roger). Used only to seed a brand-new profile so the preview is
// never blank.
export const DEFAULT_EMBLEM: EmblemState = {
	armorPrimary: 10, // cobalt
	armorSecondary: 11, // blue
	emblemPrimary: 0, // white
	emblemSecondary: 0, // white
	character: 0, // master chief
	foreground: 12, // jolly roger
	background: 3 // horizontal_split2
};

export type Appearance = Record<string, number | undefined>;

/** clamp an arbitrary byte into a valid index for a set of `count` entries. */
export function clampIndex(n: number | undefined, count: number, fallback = 0): number {
	if (typeof n !== 'number' || !Number.isFinite(n)) return fallback;
	const i = Math.round(n);
	if (i < 0) return 0;
	if (i >= count) return count - 1;
	return i;
}

/** hex for a palette index (clamped). */
export function colorHex(palette: PaletteColor[], index: number | undefined): string {
	return palette[clampIndex(index, palette.length)].hex;
}

/** name for a palette index (clamped). */
export function colorName(palette: PaletteColor[], index: number | undefined): string {
	return palette[clampIndex(index, palette.length)].name;
}

/** Decode the friendly emblem state from a raw appearance byte map, filling any
 *  unset field from `fallback` (defaults to all-zero). */
export function readEmblem(
	appearance: Appearance,
	fallback: EmblemState = ZERO_EMBLEM
): EmblemState {
	const pick = (key: string, fb: number) => {
		const v = appearance[key];
		return typeof v === 'number' && Number.isFinite(v) ? v : fb;
	};
	return {
		armorPrimary: pick(H2_KEYS.armorPrimary, fallback.armorPrimary),
		armorSecondary: pick(H2_KEYS.armorSecondary, fallback.armorSecondary),
		emblemPrimary: pick(H2_KEYS.emblemPrimary, fallback.emblemPrimary),
		emblemSecondary: pick(H2_KEYS.emblemSecondary, fallback.emblemSecondary),
		character: pick(H2_KEYS.character, fallback.character),
		foreground: pick(H2_KEYS.foreground, fallback.foreground),
		background: pick(H2_KEYS.background, fallback.background)
	};
}

const ZERO_EMBLEM: EmblemState = {
	armorPrimary: 0,
	armorSecondary: 0,
	emblemPrimary: 0,
	emblemSecondary: 0,
	character: 0,
	foreground: 0,
	background: 0
};

/** Write one emblem field back into the appearance byte map (mutates + returns
 *  it). Keyed by the EmblemState field name. */
export function setEmblemField(
	appearance: Appearance,
	field: keyof EmblemState,
	value: number
): Appearance {
	const keyByField: Record<keyof EmblemState, string> = {
		armorPrimary: H2_KEYS.armorPrimary,
		armorSecondary: H2_KEYS.armorSecondary,
		emblemPrimary: H2_KEYS.emblemPrimary,
		emblemSecondary: H2_KEYS.emblemSecondary,
		character: H2_KEYS.character,
		foreground: H2_KEYS.foreground,
		background: H2_KEYS.background
	};
	appearance[keyByField[field]] = value;
	return appearance;
}

/** True when the appearance map carries no emblem bytes yet (brand-new profile). */
export function emblemUnset(appearance: Appearance): boolean {
	return (
		appearance[H2_KEYS.foreground] === undefined &&
		appearance[H2_KEYS.background] === undefined &&
		appearance[H2_KEYS.armorPrimary] === undefined
	);
}

/** Seed a full default emblem into an empty appearance map (mutates + returns). */
export function seedEmblem(appearance: Appearance, seed: EmblemState = DEFAULT_EMBLEM): Appearance {
	(Object.keys(seed) as (keyof EmblemState)[]).forEach((f) =>
		setEmblemField(appearance, f, seed[f])
	);
	return appearance;
}

function titleCase(key: string): string {
	return key
		.replace(/(\d+)/g, ' $1')
		.split('_')
		.filter(Boolean)
		.map((w) => w.charAt(0).toUpperCase() + w.slice(1))
		.join(' ')
		.trim();
}
