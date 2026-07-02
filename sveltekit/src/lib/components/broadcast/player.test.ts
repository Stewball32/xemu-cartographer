import { describe, it, expect } from 'vitest';
import { teamArmorIndex, resolveArmorIndex, cardAppearance, hasProfileData } from './player';
import { H2_KEYS } from '$lib/utils/emblem';

describe('teamArmorIndex', () => {
	it('maps team 0/1 to the game team colours', () => {
		expect(teamArmorIndex('ce', 0)).toBe(2); // CE Red
		expect(teamArmorIndex('ce', 1)).toBe(3); // CE Blue
		expect(teamArmorIndex('h2', 0)).toBe(2); // H2 Red
		expect(teamArmorIndex('h2', 1)).toBe(11); // H2 Blue
	});
	it('wraps out-of-range team indices', () => {
		expect(teamArmorIndex('ce', 8)).toBe(teamArmorIndex('ce', 0));
		expect(teamArmorIndex('ce', -1)).toBe(teamArmorIndex('ce', 7));
	});
});

describe('resolveArmorIndex', () => {
	it('uses the shared team colour in team games (ignores per-player colour)', () => {
		expect(resolveArmorIndex('ce', true, 0, 17)).toBe(2); // team 0 → Red, not 17
		expect(resolveArmorIndex('ce', true, 1, 11)).toBe(3); // team 1 → Blue, not 11
	});
	it('uses the per-player roster colour in FFA', () => {
		expect(resolveArmorIndex('ce', false, 0, 17)).toBe(17);
		expect(resolveArmorIndex('h2', false, 3, 9)).toBe(9);
	});
});

describe('cardAppearance', () => {
	it('returns undefined for CE (no emblem system)', () => {
		expect(
			cardAppearance('ce', 2, { h2: { appearance: { [H2_KEYS.foreground]: 5 } } })
		).toBeUndefined();
	});
	it('returns undefined for an H2 player with no profile', () => {
		expect(cardAppearance('h2', 2, null)).toBeUndefined();
		expect(cardAppearance('h2', 2, {})).toBeUndefined();
		expect(cardAppearance('h2', 2, { ce: { color: 4 } })).toBeUndefined();
	});
	it('keeps the profile emblem but re-colours armor to the game-accurate index', () => {
		const appr = cardAppearance('h2', 3, {
			h2: {
				appearance: {
					[H2_KEYS.armorPrimary]: 14,
					[H2_KEYS.armorSecondary]: 15,
					[H2_KEYS.foreground]: 12,
					[H2_KEYS.background]: 7
				}
			}
		});
		expect(appr).toBeDefined();
		expect(appr![H2_KEYS.armorPrimary]).toBe(3); // overridden to team/FFA colour
		expect(appr![H2_KEYS.armorSecondary]).toBe(3);
		expect(appr![H2_KEYS.foreground]).toBe(12); // emblem preserved
		expect(appr![H2_KEYS.background]).toBe(7);
	});
});

describe('hasProfileData', () => {
	it('is false for null / empty', () => {
		expect(hasProfileData(null)).toBe(false);
		expect(hasProfileData(undefined)).toBe(false);
		expect(hasProfileData({})).toBe(false);
	});
	it('is true when CE colour or H2 appearance is present', () => {
		expect(hasProfileData({ ce: { color: 2 } })).toBe(true);
		expect(hasProfileData({ h2: { appearance: { [H2_KEYS.foreground]: 1 } } })).toBe(true);
	});
});
