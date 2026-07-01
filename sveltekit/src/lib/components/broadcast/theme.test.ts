import { describe, it, expect } from 'vitest';
import { parseGame, broadcastTheme, themeVars } from './theme';

describe('parseGame', () => {
	it('maps h2 (any case) to h2', () => {
		expect(parseGame('h2')).toBe('h2');
		expect(parseGame('H2')).toBe('h2');
	});

	it('defaults everything else to ce', () => {
		expect(parseGame('ce')).toBe('ce');
		expect(parseGame('CE')).toBe('ce');
		expect(parseGame(null)).toBe('ce');
		expect(parseGame(undefined)).toBe('ce');
		expect(parseGame('halo3')).toBe('ce');
		expect(parseGame('')).toBe('ce');
	});
});

describe('broadcastTheme', () => {
	it('returns the CE theme for ce', () => {
		const t = broadcastTheme('ce');
		expect(t.game).toBe('ce');
		expect(t.label).toBe('HALO: CE');
	});

	it('returns the H2 theme for h2', () => {
		const t = broadcastTheme('h2');
		expect(t.game).toBe('h2');
		expect(t.label).toBe('HALO 2');
	});

	it('gives the two games distinct accents (the visible difference)', () => {
		expect(broadcastTheme('ce').accent).not.toBe(broadcastTheme('h2').accent);
	});
});

describe('themeVars', () => {
	it('serialises every --bc-* custom property', () => {
		const css = themeVars(broadcastTheme('ce'));
		for (const key of [
			'--bc-accent',
			'--bc-accent2',
			'--bc-ink',
			'--bc-ink-muted',
			'--bc-panel',
			'--bc-panel-strong',
			'--bc-edge',
			'--bc-glow',
			'--bc-radius',
			'--bc-header-grad',
			'--bc-font',
			'--bc-tracking'
		]) {
			expect(css).toContain(`${key}:`);
		}
	});

	it('embeds the theme accent value', () => {
		const t = broadcastTheme('h2');
		expect(themeVars(t)).toContain(`--bc-accent:${t.accent}`);
	});
});
