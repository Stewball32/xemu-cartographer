import { describe, it, expect } from 'vitest';
import { armorAccent, armorLabel, HALO_CE_ARMOR_PALETTE } from './util';

describe('armorLabel', () => {
	it('capitalizes known palette names', () => {
		expect(armorLabel(0)).toBe('White');
		expect(armorLabel(2)).toBe('Red');
		expect(armorLabel(10)).toBe('Cobalt');
		expect(armorLabel(13)).toBe('Sage');
		expect(armorLabel(17)).toBe('Salmon');
	});

	it('returns Unknown for out-of-range indices', () => {
		expect(armorLabel(-1)).toBe('Unknown');
		expect(armorLabel(18)).toBe('Unknown');
		expect(armorLabel(99)).toBe('Unknown');
	});

	it('respects a custom palette', () => {
		const palette = ['steel', 'crimson', 'gold'] as const;
		expect(armorLabel(0, palette)).toBe('Steel');
		expect(armorLabel(2, palette)).toBe('Gold');
		expect(armorLabel(3, palette)).toBe('Unknown');
	});

	it('CE palette is the canonical 18-entry order', () => {
		expect(HALO_CE_ARMOR_PALETTE).toHaveLength(18);
		expect(HALO_CE_ARMOR_PALETTE[0]).toBe('white');
		expect(HALO_CE_ARMOR_PALETTE[17]).toBe('salmon');
	});
});

describe('armorAccent', () => {
	it('returns matching bg + dot tokens for known indices', () => {
		expect(armorAccent(0)).toEqual({ bg: 'bg-armor-white/20', dot: 'bg-armor-white' });
		expect(armorAccent(10)).toEqual({ bg: 'bg-armor-cobalt/20', dot: 'bg-armor-cobalt' });
		expect(armorAccent(17)).toEqual({ bg: 'bg-armor-salmon/20', dot: 'bg-armor-salmon' });
	});

	it('falls back to surface tokens for out-of-range indices', () => {
		expect(armorAccent(-1)).toEqual({ bg: 'bg-surface-500/20', dot: 'bg-surface-500' });
		expect(armorAccent(18)).toEqual({ bg: 'bg-surface-500/20', dot: 'bg-surface-500' });
	});
});
