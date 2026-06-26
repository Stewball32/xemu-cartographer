import { describe, it, expect } from 'vitest';
import {
	H2_COLORS,
	CE_COLORS,
	H2_BACKGROUNDS,
	H2_CHARACTERS,
	H2_KEYS,
	H2_OFFSETS,
	FOREGROUNDS,
	FOREGROUND_COUNT,
	foregroundSvg,
	clampIndex,
	colorHex,
	colorName,
	readEmblem,
	setEmblemField,
	emblemUnset,
	seedEmblem,
	DEFAULT_EMBLEM,
	type Appearance
} from './emblem';
import { backgroundSvg, BACKGROUND_COUNT } from './emblem-backgrounds';

describe('value sets (in-game enum order)', () => {
	it('has the right cardinalities', () => {
		expect(H2_COLORS).toHaveLength(18);
		expect(CE_COLORS).toHaveLength(18);
		expect(H2_BACKGROUNDS).toHaveLength(32);
		expect(H2_CHARACTERS).toHaveLength(4);
		expect(FOREGROUNDS).toHaveLength(64);
		expect(FOREGROUND_COUNT).toBe(64);
		expect(BACKGROUND_COUNT).toBe(32);
	});

	it('every palette entry is a valid hex', () => {
		for (const c of [...H2_COLORS, ...CE_COLORS]) {
			expect(c.hex).toMatch(/^#[0-9a-f]{6}$/i);
			expect(c.name.length).toBeGreaterThan(0);
		}
	});

	it('backgrounds/foregrounds carry index === position', () => {
		H2_BACKGROUNDS.forEach((b, i) => expect(b.index).toBe(i));
		FOREGROUNDS.forEach((f, i) => expect(f.index).toBe(i));
	});
});

describe('the byte-key contract matches the verified profile offsets', () => {
	it('maps keys to the confirmed 0x118 block offsets', () => {
		expect(H2_OFFSETS[H2_KEYS.armorPrimary]).toBe(0x118);
		expect(H2_OFFSETS[H2_KEYS.armorSecondary]).toBe(0x119);
		expect(H2_OFFSETS[H2_KEYS.emblemPrimary]).toBe(0x11a);
		expect(H2_OFFSETS[H2_KEYS.emblemSecondary]).toBe(0x11b);
		expect(H2_OFFSETS[H2_KEYS.character]).toBe(0x11c);
		expect(H2_OFFSETS[H2_KEYS.foreground]).toBe(0x11d);
		expect(H2_OFFSETS[H2_KEYS.background]).toBe(0x11e);
		expect(H2_OFFSETS[H2_KEYS.flags]).toBe(0x11f);
	});
});

describe('clampIndex', () => {
	it('clamps, rounds, and handles junk', () => {
		expect(clampIndex(5, 18)).toBe(5);
		expect(clampIndex(-3, 18)).toBe(0);
		expect(clampIndex(99, 18)).toBe(17);
		expect(clampIndex(2.6, 18)).toBe(3);
		expect(clampIndex(undefined, 18, 4)).toBe(4);
		expect(clampIndex(NaN, 18, 0)).toBe(0);
	});
});

describe('palette lookups clamp safely', () => {
	it('colorHex / colorName never throw on bad input', () => {
		expect(colorHex(H2_COLORS, 2)).toBe('#ea1f2f'); // red
		expect(colorName(H2_COLORS, 13)).toBe('Purple');
		expect(colorHex(H2_COLORS, 999)).toBe(H2_COLORS[17].hex);
		expect(colorName(H2_COLORS, -1)).toBe(H2_COLORS[0].name);
	});
});

describe('readEmblem', () => {
	it('falls back to defaults when unset', () => {
		expect(readEmblem({}, DEFAULT_EMBLEM)).toEqual({
			armorPrimary: DEFAULT_EMBLEM.armorPrimary,
			armorSecondary: DEFAULT_EMBLEM.armorSecondary,
			emblemPrimary: DEFAULT_EMBLEM.emblemPrimary,
			emblemSecondary: DEFAULT_EMBLEM.emblemSecondary,
			character: DEFAULT_EMBLEM.character,
			foreground: DEFAULT_EMBLEM.foreground,
			background: DEFAULT_EMBLEM.background
		});
	});

	it('reads explicit bytes (including explicit zero)', () => {
		const ap: Appearance = { [H2_KEYS.armorPrimary]: 2 };
		ap[H2_KEYS.foreground] = 0; // explicit zero must win over a non-zero fallback
		const e = readEmblem(ap, DEFAULT_EMBLEM);
		expect(e.armorPrimary).toBe(2);
		expect(e.foreground).toBe(0);
	});
});

describe('setEmblemField / seed / unset round-trip', () => {
	it('writes the correct key', () => {
		const ap: Appearance = {};
		setEmblemField(ap, 'foreground', 26);
		expect(ap[H2_KEYS.foreground]).toBe(26);
	});

	it('emblemUnset true on empty, false after seeding', () => {
		const ap: Appearance = {};
		expect(emblemUnset(ap)).toBe(true);
		seedEmblem(ap);
		expect(emblemUnset(ap)).toBe(false);
		// seeding writes a complete default emblem that reads back identically
		expect(readEmblem(ap)).toEqual({
			armorPrimary: DEFAULT_EMBLEM.armorPrimary,
			armorSecondary: DEFAULT_EMBLEM.armorSecondary,
			emblemPrimary: DEFAULT_EMBLEM.emblemPrimary,
			emblemSecondary: DEFAULT_EMBLEM.emblemSecondary,
			character: DEFAULT_EMBLEM.character,
			foreground: DEFAULT_EMBLEM.foreground,
			background: DEFAULT_EMBLEM.background
		});
	});
});

describe('art generators return valid markup for every index', () => {
	it('foregroundSvg 0..63', () => {
		for (let i = 0; i < 64; i++) {
			const s = foregroundSvg(i, '#ffffff', '#000000');
			expect(typeof s).toBe('string');
			expect(s.length).toBeGreaterThan(10);
			expect(s.trimStart().startsWith('<')).toBe(true);
		}
	});

	it('backgroundSvg 0..31', () => {
		for (let i = 0; i < 32; i++) {
			const s = backgroundSvg(i, '#112233', '#445566');
			expect(s.length).toBeGreaterThan(10);
			expect(s).toContain('<');
			// two-tone backgrounds embed both armor colors
			if (i !== 0) expect(s.toLowerCase()).toContain('#445566');
		}
	});
});
